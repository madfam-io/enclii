package builder

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/madfam-org/enclii/apps/roundhouse/internal/queue"
	"go.uber.org/zap"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

const (
	// KanikoBuildNamespace is the namespace where build jobs run
	KanikoBuildNamespace = "enclii-builds"

	// KanikoImage is the container image for Kaniko executor
	KanikoImage = "gcr.io/kaniko-project/executor:v1.19.0"

	// SyftImage is the container image for SBOM generation
	SyftImage = "anchore/syft:v1.4.1"

	// CosignImage is the container image for image signing
	CosignImage = "gcr.io/projectsigstore/cosign:v2.2.3"

	// Labels for build jobs
	LabelBuildID   = "enclii.dev/build-id"
	LabelServiceID = "enclii.dev/service-id"
	LabelAppName   = "app.kubernetes.io/name"

	// Job types for post-build operations
	JobTypeSBOM   = "sbom"
	JobTypeSigning = "signing"
)

// KanikoExecutor handles builds using Kubernetes Jobs with Kaniko
type KanikoExecutor struct {
	k8sClient      kubernetes.Interface
	registry       string
	registryUser   string
	registryPass   string
	generateSBOM   bool
	signImages     bool
	cosignKey      string
	timeout        time.Duration
	cacheRepo      string
	gitCredentials string // Secret name for git credentials
	logger         *zap.Logger
	logFunc        func(jobID uuid.UUID, line string)
}

// KanikoExecutorConfig configures the Kaniko executor
type KanikoExecutorConfig struct {
	K8sClient      kubernetes.Interface
	Registry       string
	RegistryUser   string
	RegistryPass   string
	GenerateSBOM   bool
	SignImages     bool
	CosignKey      string
	Timeout        time.Duration
	CacheRepo      string        // Optional: registry path for layer caching
	GitCredentials string        // Optional: secret name with git token
}

// NewKanikoExecutor creates a new Kaniko-based build executor
func NewKanikoExecutor(cfg *KanikoExecutorConfig, logger *zap.Logger, logFunc func(uuid.UUID, string)) *KanikoExecutor {
	cacheRepo := cfg.CacheRepo
	if cacheRepo == "" {
		cacheRepo = cfg.Registry + "/cache"
	}

	return &KanikoExecutor{
		k8sClient:      cfg.K8sClient,
		registry:       cfg.Registry,
		registryUser:   cfg.RegistryUser,
		registryPass:   cfg.RegistryPass,
		generateSBOM:   cfg.GenerateSBOM,
		signImages:     cfg.SignImages,
		cosignKey:      cfg.CosignKey,
		timeout:        cfg.Timeout,
		cacheRepo:      cacheRepo,
		gitCredentials: cfg.GitCredentials,
		logger:         logger,
		logFunc:        logFunc,
	}
}

// Execute runs the build using a Kubernetes Job with Kaniko
func (e *KanikoExecutor) Execute(ctx context.Context, job *queue.BuildJob) (*queue.BuildResult, error) {
	startTime := time.Now()

	result := &queue.BuildResult{
		JobID:     job.ID,
		ReleaseID: job.ReleaseID,
	}

	e.log(job.ID, "📦 Starting Kaniko build for %s @ %s", job.GitRepo, job.GitSHA[:8])

	// Generate image tag
	imageTag := e.generateImageTag(job)
	result.ImageURI = imageTag

	// Create the Kubernetes Job
	k8sJob, err := e.createBuildJob(ctx, job, imageTag)
	if err != nil {
		return e.failResult(result, startTime, "failed to create build job: %v", err)
	}

	e.log(job.ID, "🚀 Created Kubernetes Job: %s", k8sJob.Name)

	// Watch for job completion
	err = e.watchJobCompletion(ctx, job.ID, k8sJob.Name)
	if err != nil {
		// Try to get logs before failing
		e.streamJobLogs(ctx, job.ID, k8sJob.Name)
		return e.failResult(result, startTime, "build failed: %v", err)
	}

	e.log(job.ID, "✅ Kaniko build completed successfully")

	// Get final logs
	e.streamJobLogs(ctx, job.ID, k8sJob.Name)

	// Get image digest from registry (post-push)
	// Note: Kaniko pushes directly, so we need to query the registry
	digest, err := e.getImageDigestFromRegistry(ctx, imageTag)
	if err != nil {
		e.logger.Warn("failed to get image digest", zap.Error(err))
	} else {
		result.ImageDigest = digest
	}

	// Generate SBOM (run as separate job if enabled)
	if e.generateSBOM {
		e.log(job.ID, "📋 Generating SBOM...")
		sbom, format, err := e.runSBOMGeneration(ctx, job.ID, imageTag)
		if err != nil {
			e.logger.Warn("failed to generate SBOM", zap.Error(err))
		} else {
			result.SBOM = sbom
			result.SBOMFormat = format
			e.log(job.ID, "✅ SBOM generated (%s)", format)
		}
	}

	// Sign image (run as separate job if enabled)
	if e.signImages && e.cosignKey != "" {
		e.log(job.ID, "🔐 Signing image...")
		signature, err := e.runImageSigning(ctx, job.ID, imageTag)
		if err != nil {
			e.logger.Warn("failed to sign image", zap.Error(err))
		} else {
			result.ImageSignature = signature
			e.log(job.ID, "✅ Image signed")
		}
	}

	result.Success = true
	result.DurationSecs = time.Since(startTime).Seconds()

	e.log(job.ID, "🎉 Build completed in %.1fs", result.DurationSecs)

	return result, nil
}

// createBuildJob creates a Kubernetes Job for the Kaniko build
func (e *KanikoExecutor) createBuildJob(ctx context.Context, job *queue.BuildJob, imageTag string) (*batchv1.Job, error) {
	// Build Kaniko args
	args := e.buildKanikoArgs(job, imageTag)

	// Job configuration
	backoffLimit := int32(0)            // Don't retry failed builds
	ttlSeconds := int32(3600)           // Clean up after 1 hour
	activeDeadlineSeconds := int64(e.timeout.Seconds())

	// Security context - Kaniko MUST run as root (UID 0) to unpack container filesystem layers.
	// When building images, Kaniko needs to create directories like /bin, /usr, etc. which
	// are owned by root. This is safe because Kaniko runs in an unprivileged container
	// (no elevated host capabilities), it just needs root within the container namespace.
	runAsNonRoot := false
	runAsUser := int64(0)
	runAsGroup := int64(0)
	fsGroup := int64(0)

	k8sJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("build-%s", job.ID.String()[:8]),
			Namespace: KanikoBuildNamespace,
			Labels: map[string]string{
				LabelBuildID:   job.ID.String(),
				LabelServiceID: job.ServiceID.String(),
				LabelAppName:   "kaniko-build",
			},
			Annotations: map[string]string{
				"enclii.dev/git-repo":   job.GitRepo,
				"enclii.dev/git-sha":    job.GitSHA,
				"enclii.dev/git-branch": job.GitBranch,
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoffLimit,
			TTLSecondsAfterFinished: &ttlSeconds,
			ActiveDeadlineSeconds:   &activeDeadlineSeconds,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						LabelBuildID: job.ID.String(),
						LabelAppName: "kaniko-build",
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: &runAsNonRoot,
						RunAsUser:    &runAsUser,
						RunAsGroup:   &runAsGroup,
						FSGroup:      &fsGroup,
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
					// Avoid GPU nodes
					Affinity: &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							PreferredDuringSchedulingIgnoredDuringExecution: []corev1.PreferredSchedulingTerm{
								{
									Weight: 100,
									Preference: corev1.NodeSelectorTerm{
										MatchExpressions: []corev1.NodeSelectorRequirement{
											{
												Key:      "nvidia.com/gpu",
												Operator: corev1.NodeSelectorOpDoesNotExist,
											},
										},
									},
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:  "kaniko",
							Image: KanikoImage,
							Args:  args,
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("500m"),
									corev1.ResourceMemory: resource.MustParse("1Gi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("2"),
									corev1.ResourceMemory: resource.MustParse("4Gi"),
								},
							},
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: boolPtr(false),
								ReadOnlyRootFilesystem:   boolPtr(false), // Kaniko needs writable /kaniko
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"ALL"},
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "docker-config",
									MountPath: "/kaniko/.docker",
									ReadOnly:  true,
								},
							},
							Env: e.buildEnvVars(job),
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "docker-config",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "regcred",
									Items: []corev1.KeyToPath{
										{
											Key:  ".dockerconfigjson",
											Path: "config.json",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	// Add git credentials volume if configured
	if e.gitCredentials != "" {
		k8sJob.Spec.Template.Spec.Volumes = append(k8sJob.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: "git-credentials",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: e.gitCredentials,
				},
			},
		})
	}

	return e.k8sClient.BatchV1().Jobs(KanikoBuildNamespace).Create(ctx, k8sJob, metav1.CreateOptions{})
}

// buildKanikoArgs constructs the Kaniko executor arguments
func (e *KanikoExecutor) buildKanikoArgs(job *queue.BuildJob, imageTag string) []string {
	dockerfile := job.BuildConfig.Dockerfile
	if dockerfile == "" {
		dockerfile = "Dockerfile"
	}

	contextPath := job.BuildConfig.Context
	if contextPath == "" {
		contextPath = "."
	}

	// Git context URL format: git://[repository]#[ref]#[commit-sha]
	// Strip https:// or http:// prefix from repo URL if present
	repoURL := job.GitRepo
	repoURL = strings.TrimPrefix(repoURL, "https://")
	repoURL = strings.TrimPrefix(repoURL, "http://")
	gitContext := fmt.Sprintf("git://%s#refs/heads/%s#%s",
		repoURL, job.GitBranch, job.GitSHA)

	// If context is a subdirectory, append to git context
	if contextPath != "." {
		gitContext = gitContext + ":" + contextPath
	}

	args := []string{
		"--dockerfile=" + dockerfile,
		"--context=" + gitContext,
		"--destination=" + imageTag,
		"--destination=" + e.generateLatestTag(job),
		// Layer caching
		"--cache=true",
		"--cache-repo=" + e.cacheRepo,
		"--cache-ttl=168h", // 7 days
		// Reproducibility
		"--reproducible",
		"--snapshot-mode=redo",
		// Build metadata
		"--label=org.opencontainers.image.source=" + job.GitRepo,
		"--label=org.opencontainers.image.revision=" + job.GitSHA,
		"--label=org.opencontainers.image.created=" + time.Now().UTC().Format(time.RFC3339),
		"--label=io.enclii.service-id=" + job.ServiceID.String(),
		"--label=io.enclii.release-id=" + job.ReleaseID.String(),
		// Verbosity
		"--verbosity=info",
	}

	// Add build args
	for key, value := range job.BuildConfig.BuildArgs {
		args = append(args, fmt.Sprintf("--build-arg=%s=%s", key, value))
	}

	// Add target if specified (multi-stage builds)
	if job.BuildConfig.Target != "" {
		args = append(args, "--target="+job.BuildConfig.Target)
	}

	return args
}

// buildEnvVars constructs environment variables for the build
func (e *KanikoExecutor) buildEnvVars(job *queue.BuildJob) []corev1.EnvVar {
	envVars := []corev1.EnvVar{}

	// Add git token if credentials secret exists
	if e.gitCredentials != "" {
		envVars = append(envVars, corev1.EnvVar{
			Name: "GIT_TOKEN",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: e.gitCredentials,
					},
					Key:      "token",
					Optional: boolPtr(true),
				},
			},
		})
	}

	return envVars
}

// watchJobCompletion watches the Kubernetes Job until completion or timeout
func (e *KanikoExecutor) watchJobCompletion(ctx context.Context, buildID uuid.UUID, jobName string) error {
	watcher, err := e.k8sClient.BatchV1().Jobs(KanikoBuildNamespace).Watch(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.name=%s", jobName),
	})
	if err != nil {
		return fmt.Errorf("failed to watch job: %w", err)
	}
	defer watcher.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-watcher.ResultChan():
			if !ok {
				return fmt.Errorf("watch channel closed")
			}

			if event.Type == watch.Error {
				return fmt.Errorf("watch error")
			}

			k8sJob, ok := event.Object.(*batchv1.Job)
			if !ok {
				continue
			}

			// Check for completion
			for _, condition := range k8sJob.Status.Conditions {
				if condition.Type == batchv1.JobComplete && condition.Status == corev1.ConditionTrue {
					return nil
				}
				if condition.Type == batchv1.JobFailed && condition.Status == corev1.ConditionTrue {
					return fmt.Errorf("job failed: %s", condition.Message)
				}
			}
		}
	}
}

// streamJobLogs streams logs from the build pod
func (e *KanikoExecutor) streamJobLogs(ctx context.Context, buildID uuid.UUID, jobName string) {
	// Find the pod for this job
	pods, err := e.k8sClient.CoreV1().Pods(KanikoBuildNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("job-name=%s", jobName),
	})
	if err != nil || len(pods.Items) == 0 {
		e.logger.Warn("could not find pod for job", zap.String("job", jobName))
		return
	}

	podName := pods.Items[0].Name

	// Get logs
	req := e.k8sClient.CoreV1().Pods(KanikoBuildNamespace).GetLogs(podName, &corev1.PodLogOptions{
		Container: "kaniko",
	})

	logs, err := req.Stream(ctx)
	if err != nil {
		e.logger.Warn("could not stream logs", zap.Error(err))
		return
	}
	defer logs.Close()

	// Read and emit logs
	buf := make([]byte, 4096)
	for {
		n, err := logs.Read(buf)
		if n > 0 {
			lines := strings.Split(string(buf[:n]), "\n")
			for _, line := range lines {
				if line != "" {
					e.log(buildID, "%s", line)
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			e.logger.Warn("error reading logs", zap.Error(err))
			break
		}
	}
}

// getImageDigestFromRegistry queries the registry for the image digest
func (e *KanikoExecutor) getImageDigestFromRegistry(ctx context.Context, imageTag string) (string, error) {
	// For now, return empty - full implementation would use crane or skopeo
	// to query the registry for the manifest digest
	return "", nil
}

// runSBOMGeneration runs Syft to generate SBOM for the image
func (e *KanikoExecutor) runSBOMGeneration(ctx context.Context, buildID uuid.UUID, imageTag string) (string, string, error) {
	jobName := fmt.Sprintf("sbom-%s", buildID.String()[:8])
	format := "spdx-json" // SPDX is widely supported and recommended

	// Security context - run as non-root
	runAsNonRoot := true
	runAsUser := int64(1000)
	runAsGroup := int64(1000)
	fsGroup := int64(1000)

	// Job configuration
	backoffLimit := int32(0)
	ttlSeconds := int32(1800) // Clean up after 30 minutes
	activeDeadlineSeconds := int64(300) // 5 minute timeout for SBOM

	k8sJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: KanikoBuildNamespace,
			Labels: map[string]string{
				LabelBuildID: buildID.String(),
				LabelAppName: "syft-sbom",
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoffLimit,
			TTLSecondsAfterFinished: &ttlSeconds,
			ActiveDeadlineSeconds:   &activeDeadlineSeconds,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						LabelBuildID: buildID.String(),
						LabelAppName: "syft-sbom",
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: &runAsNonRoot,
						RunAsUser:    &runAsUser,
						RunAsGroup:   &runAsGroup,
						FSGroup:      &fsGroup,
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
					Containers: []corev1.Container{
						{
							Name:  "syft",
							Image: SyftImage,
							Args: []string{
								"scan",
								"--output", format,
								"registry:" + imageTag,
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("256Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("500m"),
									corev1.ResourceMemory: resource.MustParse("1Gi"),
								},
							},
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: boolPtr(false),
								ReadOnlyRootFilesystem:   boolPtr(true),
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"ALL"},
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "docker-config",
									MountPath: "/home/syft/.docker",
									ReadOnly:  true,
								},
								{
									Name:      "tmp",
									MountPath: "/tmp",
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "DOCKER_CONFIG",
									Value: "/home/syft/.docker",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "docker-config",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "regcred",
									Items: []corev1.KeyToPath{
										{
											Key:  ".dockerconfigjson",
											Path: "config.json",
										},
									},
								},
							},
						},
						{
							Name: "tmp",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			},
		},
	}

	// Create the job
	_, err := e.k8sClient.BatchV1().Jobs(KanikoBuildNamespace).Create(ctx, k8sJob, metav1.CreateOptions{})
	if err != nil {
		return "", "", fmt.Errorf("failed to create SBOM job: %w", err)
	}

	e.log(buildID, "📋 Created SBOM generation job: %s", jobName)

	// Wait for completion
	if err := e.watchJobCompletion(ctx, buildID, jobName); err != nil {
		return "", "", fmt.Errorf("SBOM generation failed: %w", err)
	}

	// Get SBOM output from job logs
	sbom, err := e.getJobOutput(ctx, jobName)
	if err != nil {
		return "", "", fmt.Errorf("failed to get SBOM output: %w", err)
	}

	return sbom, format, nil
}

// runImageSigning runs Cosign to sign the image
func (e *KanikoExecutor) runImageSigning(ctx context.Context, buildID uuid.UUID, imageTag string) (string, error) {
	jobName := fmt.Sprintf("sign-%s", buildID.String()[:8])

	// Security context - run as non-root
	runAsNonRoot := true
	runAsUser := int64(1000)
	runAsGroup := int64(1000)
	fsGroup := int64(1000)

	// Job configuration
	backoffLimit := int32(0)
	ttlSeconds := int32(1800) // Clean up after 30 minutes
	activeDeadlineSeconds := int64(180) // 3 minute timeout for signing

	// Cosign supports keyless signing via Fulcio/Rekor or key-based signing
	// We support both modes based on configuration
	var args []string
	var envVars []corev1.EnvVar
	var volumes []corev1.Volume
	var volumeMounts []corev1.VolumeMount

	// Registry credentials for pulling/pushing signatures
	volumes = append(volumes, corev1.Volume{
		Name: "docker-config",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: "regcred",
				Items: []corev1.KeyToPath{
					{
						Key:  ".dockerconfigjson",
						Path: "config.json",
					},
				},
			},
		},
	})
	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		Name:      "docker-config",
		MountPath: "/home/nonroot/.docker",
		ReadOnly:  true,
	})
	envVars = append(envVars, corev1.EnvVar{
		Name:  "DOCKER_CONFIG",
		Value: "/home/nonroot/.docker",
	})

	if e.cosignKey != "" {
		// Key-based signing - mount the signing key secret
		args = []string{
			"sign",
			"--key", "/cosign/cosign.key",
			"--yes", // Skip confirmation
			imageTag,
		}

		volumes = append(volumes, corev1.Volume{
			Name: "cosign-key",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: e.cosignKey,
				},
			},
		})
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "cosign-key",
			MountPath: "/cosign",
			ReadOnly:  true,
		})

		// Cosign password for the key (if encrypted)
		envVars = append(envVars, corev1.EnvVar{
			Name: "COSIGN_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: e.cosignKey,
					},
					Key:      "password",
					Optional: boolPtr(true),
				},
			},
		})
	} else {
		// Keyless signing using Fulcio and Rekor (OIDC-based)
		args = []string{
			"sign",
			"--yes", // Skip confirmation
			imageTag,
		}

		// Enable experimental features for keyless signing
		envVars = append(envVars, corev1.EnvVar{
			Name:  "COSIGN_EXPERIMENTAL",
			Value: "1",
		})
	}

	// Tmp directory for Cosign operations
	volumes = append(volumes, corev1.Volume{
		Name: "tmp",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	})
	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		Name:      "tmp",
		MountPath: "/tmp",
	})

	k8sJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: KanikoBuildNamespace,
			Labels: map[string]string{
				LabelBuildID: buildID.String(),
				LabelAppName: "cosign-sign",
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoffLimit,
			TTLSecondsAfterFinished: &ttlSeconds,
			ActiveDeadlineSeconds:   &activeDeadlineSeconds,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						LabelBuildID: buildID.String(),
						LabelAppName: "cosign-sign",
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: &runAsNonRoot,
						RunAsUser:    &runAsUser,
						RunAsGroup:   &runAsGroup,
						FSGroup:      &fsGroup,
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
					Containers: []corev1.Container{
						{
							Name:  "cosign",
							Image: CosignImage,
							Args:  args,
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("50m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("200m"),
									corev1.ResourceMemory: resource.MustParse("512Mi"),
								},
							},
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: boolPtr(false),
								ReadOnlyRootFilesystem:   boolPtr(true),
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"ALL"},
								},
							},
							Env:          envVars,
							VolumeMounts: volumeMounts,
						},
					},
					Volumes: volumes,
				},
			},
		},
	}

	// Create the job
	_, err := e.k8sClient.BatchV1().Jobs(KanikoBuildNamespace).Create(ctx, k8sJob, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to create signing job: %w", err)
	}

	e.log(buildID, "🔐 Created image signing job: %s", jobName)

	// Wait for completion
	if err := e.watchJobCompletion(ctx, buildID, jobName); err != nil {
		return "", fmt.Errorf("image signing failed: %w", err)
	}

	// For Cosign, the signature is stored in the registry alongside the image
	// Return a reference to indicate signing was successful
	signature := fmt.Sprintf("%s.sig", imageTag)
	return signature, nil
}

// generateImageTag generates the full image tag
func (e *KanikoExecutor) generateImageTag(job *queue.BuildJob) string {
	shortSHA := job.GitSHA
	if len(shortSHA) > 8 {
		shortSHA = shortSHA[:8]
	}

	return fmt.Sprintf("%s/%s/%s:%s",
		e.registry,
		job.ProjectID.String()[:8],
		job.ServiceID.String()[:8],
		shortSHA,
	)
}

// generateLatestTag generates the :latest tag variant
func (e *KanikoExecutor) generateLatestTag(job *queue.BuildJob) string {
	return fmt.Sprintf("%s/%s/%s:latest",
		e.registry,
		job.ProjectID.String()[:8],
		job.ServiceID.String()[:8],
	)
}

func (e *KanikoExecutor) log(jobID uuid.UUID, format string, args ...interface{}) {
	line := fmt.Sprintf(format, args...)
	e.logger.Info(line, zap.String("job_id", jobID.String()))
	if e.logFunc != nil {
		e.logFunc(jobID, line)
	}
}

func (e *KanikoExecutor) failResult(result *queue.BuildResult, startTime time.Time, format string, args ...interface{}) (*queue.BuildResult, error) {
	result.Success = false
	result.ErrorMessage = fmt.Sprintf(format, args...)
	result.DurationSecs = time.Since(startTime).Seconds()
	e.log(result.JobID, "❌ %s", result.ErrorMessage)
	return result, fmt.Errorf("%s", result.ErrorMessage)
}

// getJobOutput retrieves the stdout from a completed job
func (e *KanikoExecutor) getJobOutput(ctx context.Context, jobName string) (string, error) {
	// Find the pod for this job
	pods, err := e.k8sClient.CoreV1().Pods(KanikoBuildNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("job-name=%s", jobName),
	})
	if err != nil || len(pods.Items) == 0 {
		return "", fmt.Errorf("could not find pod for job %s", jobName)
	}

	podName := pods.Items[0].Name

	// Determine container name based on job type
	containerName := "syft" // default
	if strings.HasPrefix(jobName, "sign-") {
		containerName = "cosign"
	}

	// Get logs (stdout)
	req := e.k8sClient.CoreV1().Pods(KanikoBuildNamespace).GetLogs(podName, &corev1.PodLogOptions{
		Container: containerName,
	})

	logs, err := req.Stream(ctx)
	if err != nil {
		return "", fmt.Errorf("could not stream logs: %w", err)
	}
	defer logs.Close()

	// Read all output
	var output strings.Builder
	buf := make([]byte, 4096)
	for {
		n, err := logs.Read(buf)
		if n > 0 {
			output.Write(buf[:n])
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("error reading logs: %w", err)
		}
	}

	return output.String(), nil
}

// Helper functions
func boolPtr(b bool) *bool {
	return &b
}
