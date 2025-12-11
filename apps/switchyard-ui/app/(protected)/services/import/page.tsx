'use client';

import { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { apiGet } from "@/lib/api";
import type { GitHubRepository, GitHubReposResponse, IntegrationStatus } from "@/lib/types";

// Icons as SVG components since lucide-react may not be installed
const GithubIcon = () => (
  <svg className="h-5 w-5" viewBox="0 0 24 24" fill="currentColor">
    <path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z"/>
  </svg>
);

const SearchIcon = () => (
  <svg className="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
  </svg>
);

const ExternalLinkIcon = () => (
  <svg className="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
  </svg>
);

const LockIcon = () => (
  <svg className="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
  </svg>
);

const GlobeIcon = () => (
  <svg className="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3.055 11H5a2 2 0 012 2v1a2 2 0 002 2 2 2 0 012 2v2.945M8 3.935V5.5A2.5 2.5 0 0010.5 8h.5a2 2 0 012 2 2 2 0 104 0 2 2 0 012-2h1.064M15 20.488V18a2 2 0 012-2h3.064M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
  </svg>
);

const ClockIcon = () => (
  <svg className="h-3 w-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
  </svg>
);

export default function ImportRepositoryPage() {
  const router = useRouter();
  const [repos, setRepos] = useState<GitHubRepository[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchTerm, setSearchTerm] = useState("");
  const [githubStatus, setGithubStatus] = useState<IntegrationStatus | null>(null);
  const [linkingGitHub, setLinkingGitHub] = useState(false);

  // Check GitHub integration status first
  useEffect(() => {
    const checkGitHubStatus = async () => {
      try {
        const status = await apiGet<IntegrationStatus>('/v1/integrations/github/status');
        setGithubStatus(status);

        if (status.linked && status.can_access_repos) {
          fetchRepositories();
        } else {
          setLoading(false);
        }
      } catch (err) {
        console.error("Failed to check GitHub status:", err);
        setError("Unable to verify GitHub connection");
        setLoading(false);
      }
    };
    checkGitHubStatus();
  }, []);

  const fetchRepositories = async () => {
    try {
      setError(null);
      const data = await apiGet<GitHubReposResponse>('/v1/integrations/github/repos');
      setRepos(data.repositories || []);
      setLoading(false);
    } catch (err) {
      console.error("Failed to fetch repositories:", err);
      setError(err instanceof Error ? err.message : "Failed to load repositories");
      setLoading(false);
    }
  };

  const handleSelectRepo = (repo: GitHubRepository) => {
    // Navigate to the analyze step which will detect services in the repo
    router.push(`/services/import/${repo.owner.login}/${repo.name}?branch=${repo.default_branch}`);
  };

  // Handle GitHub OAuth linking via Janua
  const handleConnectGitHub = async () => {
    const januaUrl = process.env.NEXT_PUBLIC_JANUA_URL || 'https://auth.madfam.io';
    const redirectUri = window.location.href;

    setLinkingGitHub(true);
    setError(null);

    try {
      // Get tokens from localStorage
      const storedTokens = localStorage.getItem("enclii_tokens");
      if (!storedTokens) {
        throw new Error("Not authenticated. Please log in again.");
      }

      const tokens = JSON.parse(storedTokens);

      // Use IDP token (Janua token) for calling Janua APIs
      // Fall back to accessToken for backwards compatibility (won't work with Janua but might work locally)
      const idpToken = tokens.idpToken || tokens.accessToken;
      if (!idpToken) {
        throw new Error("No authentication token found. Please log in again.");
      }

      // If we don't have an IDP token, the user needs to re-authenticate via OIDC
      if (!tokens.idpToken) {
        console.warn("No IDP token available - user may need to re-authenticate via OIDC to get Janua token");
        // Try anyway with access token - it will fail if Janua doesn't accept it
      }

      // Call Janua's OAuth link endpoint (POST)
      const response = await fetch(
        `${januaUrl}/api/v1/auth/oauth/link/github?redirect_uri=${encodeURIComponent(redirectUri)}`,
        {
          method: 'POST',
          headers: {
            'Authorization': `Bearer ${idpToken}`,
            'Content-Type': 'application/json',
          },
        }
      );

      if (!response.ok) {
        const errorData = await response.json().catch(() => ({}));
        // If we get 401 and don't have IDP token, suggest re-login
        if (response.status === 401 && !tokens.idpToken) {
          throw new Error("Session expired. Please log out and log back in to refresh your authentication.");
        }
        throw new Error(errorData.detail || errorData.message || `Failed to initiate GitHub linking: ${response.status}`);
      }

      const data = await response.json();

      if (data.authorization_url) {
        // Redirect to GitHub OAuth
        window.location.href = data.authorization_url;
      } else {
        throw new Error("No authorization URL returned from server");
      }
    } catch (err) {
      console.error("Failed to connect GitHub:", err);
      setError(err instanceof Error ? err.message : "Failed to connect GitHub");
      setLinkingGitHub(false);
    }
  };

  const filteredRepos = repos.filter(repo =>
    repo.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
    repo.full_name.toLowerCase().includes(searchTerm.toLowerCase()) ||
    (repo.description?.toLowerCase().includes(searchTerm.toLowerCase()))
  );

  // GitHub not connected state
  if (!loading && githubStatus && !githubStatus.linked) {
    return (
      <div className="container mx-auto py-8 max-w-4xl">
        <div className="mb-6">
          <Link href="/services" className="text-blue-600 hover:text-blue-800 text-sm flex items-center gap-1">
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
            </svg>
            Back to Services
          </Link>
        </div>
        <Card className="border-yellow-200 bg-yellow-50">
          <CardContent className="py-12 text-center">
            <div className="mx-auto mb-4 h-16 w-16 text-gray-400">
              <GithubIcon />
            </div>
            <h2 className="text-xl font-semibold mb-2">Connect GitHub</h2>
            <p className="text-gray-600 mb-6">
              Link your GitHub account to import repositories and enable auto-deployments.
            </p>
            <Button
              onClick={handleConnectGitHub}
              disabled={linkingGitHub}
              className="inline-flex items-center gap-2"
            >
              {linkingGitHub ? (
                <>
                  <svg className="animate-spin h-5 w-5" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                    <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                    <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                  </svg>
                  Connecting...
                </>
              ) : (
                <>
                  <GithubIcon />
                  Connect GitHub via Janua
                </>
              )}
            </Button>
            {error && (
              <p className="text-red-600 text-sm mt-4">{error}</p>
            )}
          </CardContent>
        </Card>
      </div>
    );
  }

  return (
    <div className="container mx-auto py-8">
      {/* Header */}
      <div className="flex items-center justify-between mb-8">
        <div>
          <Link href="/services" className="text-blue-600 hover:text-blue-800 text-sm mb-2 inline-flex items-center gap-1">
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
            </svg>
            Back to Services
          </Link>
          <h1 className="text-3xl font-bold flex items-center gap-3">
            <GithubIcon />
            Import from GitHub
          </h1>
          <p className="text-muted-foreground mt-2">
            Select a repository to deploy as a service
          </p>
        </div>
        {githubStatus?.provider_email && (
          <Badge variant="outline" className="flex items-center gap-2">
            <div className="w-2 h-2 bg-green-500 rounded-full" />
            {githubStatus.provider_email}
          </Badge>
        )}
      </div>

      {/* Search */}
      <div className="relative mb-6">
        <div className="absolute left-3 top-1/2 transform -translate-y-1/2 text-gray-400">
          <SearchIcon />
        </div>
        <Input
          placeholder="Search repositories..."
          value={searchTerm}
          onChange={(e) => setSearchTerm(e.target.value)}
          className="pl-10"
        />
      </div>

      {/* Loading State */}
      {loading && (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {[1, 2, 3, 4].map((i) => (
            <Card key={i} className="animate-pulse">
              <CardContent className="py-6">
                <div className="h-4 bg-gray-200 rounded w-3/4 mb-2" />
                <div className="h-3 bg-gray-100 rounded w-1/2" />
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      {/* Error State */}
      {error && (
        <Card className="border-red-200 bg-red-50">
          <CardContent className="py-6 text-center">
            <p className="text-red-600">{error}</p>
            <Button variant="outline" onClick={fetchRepositories} className="mt-4">
              Retry
            </Button>
          </CardContent>
        </Card>
      )}

      {/* Repository List */}
      {!loading && !error && (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {filteredRepos.map((repo) => (
            <Card
              key={repo.id}
              className="hover:border-blue-300 cursor-pointer transition-colors"
              onClick={() => handleSelectRepo(repo)}
            >
              <CardHeader className="pb-2">
                <div className="flex items-start justify-between">
                  <div className="flex items-center gap-2">
                    <img
                      src={repo.owner.avatar_url}
                      alt={repo.owner.login}
                      className="w-6 h-6 rounded-full"
                    />
                    <CardTitle className="text-base">
                      {repo.full_name}
                    </CardTitle>
                  </div>
                  <div className="flex items-center gap-2">
                    {repo.private ? (
                      <span className="text-gray-400"><LockIcon /></span>
                    ) : (
                      <span className="text-gray-400"><GlobeIcon /></span>
                    )}
                    <a
                      href={repo.html_url}
                      target="_blank"
                      rel="noopener noreferrer"
                      onClick={(e) => e.stopPropagation()}
                      className="text-gray-400 hover:text-blue-600"
                    >
                      <ExternalLinkIcon />
                    </a>
                  </div>
                </div>
              </CardHeader>
              <CardContent>
                <CardDescription className="line-clamp-2 mb-3">
                  {repo.description || "No description"}
                </CardDescription>
                <div className="flex items-center gap-3 text-sm text-muted-foreground">
                  {repo.language && (
                    <Badge variant="secondary">{repo.language}</Badge>
                  )}
                  <span className="flex items-center gap-1">
                    <ClockIcon />
                    {new Date(repo.updated_at).toLocaleDateString()}
                  </span>
                  <Badge variant="outline">{repo.default_branch}</Badge>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      {/* Empty State */}
      {!loading && !error && filteredRepos.length === 0 && (
        <Card>
          <CardContent className="py-12 text-center">
            <div className="mx-auto mb-4 h-12 w-12 text-gray-400">
              <GithubIcon />
            </div>
            <p className="text-gray-600">
              {searchTerm
                ? `No repositories matching "${searchTerm}"`
                : "No repositories found"}
            </p>
          </CardContent>
        </Card>
      )}
    </div>
  );
}
