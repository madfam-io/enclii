package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/madfam/enclii/apps/switchyard-api/internal/auth"
	"github.com/madfam/enclii/apps/switchyard-api/internal/cache"
	"github.com/madfam/enclii/apps/switchyard-api/internal/config"
	"github.com/madfam/enclii/apps/switchyard-api/internal/db"
	"github.com/madfam/enclii/apps/switchyard-api/internal/logging"
	"github.com/madfam/enclii/packages/sdk-go/pkg/types"
)

// Mock implementations for testing
type MockProjectRepository struct {
	mock.Mock
}

func (m *MockProjectRepository) Create(ctx context.Context, project *types.Project) error {
	args := m.Called(ctx, project)
	return args.Error(0)
}

func (m *MockProjectRepository) GetByID(ctx context.Context, id string) (*types.Project, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(*types.Project), args.Error(1)
}

func (m *MockProjectRepository) GetBySlug(ctx context.Context, slug string) (*types.Project, error) {
	args := m.Called(ctx, slug)
	return args.Get(0).(*types.Project), args.Error(1)
}

func (m *MockProjectRepository) List(ctx context.Context) ([]*types.Project, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*types.Project), args.Error(1)
}

func (m *MockProjectRepository) Update(ctx context.Context, project *types.Project) error {
	args := m.Called(ctx, project)
	return args.Error(0)
}

func (m *MockProjectRepository) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

type MockCacheService struct {
	mock.Mock
}

func (m *MockCacheService) Get(ctx context.Context, key string) ([]byte, error) {
	args := m.Called(ctx, key)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockCacheService) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	args := m.Called(ctx, key, value, ttl)
	return args.Error(0)
}

func (m *MockCacheService) Del(ctx context.Context, keys ...string) error {
	args := m.Called(ctx, keys)
	return args.Error(0)
}

func (m *MockCacheService) DelByTag(ctx context.Context, tag string) error {
	args := m.Called(ctx, tag)
	return args.Error(0)
}

func (m *MockCacheService) GetOrSet(ctx context.Context, key string, ttl time.Duration, fetchFunc func() (interface{}, error)) ([]byte, error) {
	args := m.Called(ctx, key, ttl, fetchFunc)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockCacheService) Ping(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) Debug(ctx context.Context, msg string, fields ...logging.Field) {
	m.Called(ctx, msg, fields)
}

func (m *MockLogger) Info(ctx context.Context, msg string, fields ...logging.Field) {
	m.Called(ctx, msg, fields)
}

func (m *MockLogger) Warn(ctx context.Context, msg string, fields ...logging.Field) {
	m.Called(ctx, msg, fields)
}

func (m *MockLogger) Error(ctx context.Context, msg string, fields ...logging.Field) {
	m.Called(ctx, msg, fields)
}

func (m *MockLogger) Fatal(ctx context.Context, msg string, fields ...logging.Field) {
	m.Called(ctx, msg, fields)
}

func (m *MockLogger) WithField(key string, value interface{}) logging.Logger {
	return m
}

func (m *MockLogger) WithFields(fields logging.Fields) logging.Logger {
	return m
}

func (m *MockLogger) WithError(err error) logging.Logger {
	return m
}

func (m *MockLogger) WithContext(ctx context.Context) logging.Logger {
	return m
}

func setupTestHandler() (*Handler, *MockProjectRepository, *MockCacheService, *MockLogger) {
	gin.SetMode(gin.TestMode)
	
	mockProjectRepo := &MockProjectRepository{}
	mockCache := &MockCacheService{}
	mockLogger := &MockLogger{}

	repos := &db.Repositories{
		Project: mockProjectRepo,
	}
	
	cfg := &config.Config{
		Registry: "test-registry",
	}

	handler := NewHandler(
		repos,
		cfg,
		nil, // auth manager - not needed for basic tests
		mockCache,
		nil, // builder - not needed for basic tests
		nil, // k8s client - not needed for basic tests
		nil, // controller - not needed for basic tests
		nil, // reconciler - not needed for basic tests
		nil, // metrics - not needed for basic tests
		mockLogger,
		nil, // validator - not needed for basic tests
	)

	return handler, mockProjectRepo, mockCache, mockLogger
}

func TestCreateProject(t *testing.T) {
	handler, mockRepo, mockCache, mockLogger := setupTestHandler()

	// Test successful project creation
	t.Run("successful creation", func(t *testing.T) {
		project := &types.Project{
			ID:          "test-id",
			Name:        "Test Project",
			Slug:        "test-project",
			Description: "A test project",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*types.Project")).Return(nil)
		mockCache.On("DelByTag", mock.Anything, "projects").Return(nil)
		mockLogger.On("Info", mock.Anything, mock.Anything, mock.Anything).Return()

		// Create request
		reqBody := map[string]string{
			"name":        project.Name,
			"slug":        project.Slug,
			"description": project.Description,
		}
		jsonBody, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/v1/projects", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		// Setup Gin context
		c, _ := gin.CreateTestContext(w)
		c.Request = req

		// Call handler
		handler.CreateProject(c)

		// Assertions
		assert.Equal(t, http.StatusCreated, w.Code)
		mockRepo.AssertExpectations(t)
		mockCache.AssertExpectations(t)
	})

	// Test validation error
	t.Run("validation error", func(t *testing.T) {
		mockLogger.On("Error", mock.Anything, mock.Anything, mock.Anything).Return()

		// Create request with missing required field
		reqBody := map[string]string{
			"name": "Test Project",
			// missing slug
		}
		jsonBody, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/v1/projects", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		c, _ := gin.CreateTestContext(w)
		c.Request = req

		handler.CreateProject(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestListProjects(t *testing.T) {
	handler, mockRepo, _, _ := setupTestHandler()

	t.Run("successful list", func(t *testing.T) {
		projects := []*types.Project{
			{
				ID:          "1",
				Name:        "Project 1",
				Slug:        "project-1",
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			},
			{
				ID:          "2", 
				Name:        "Project 2",
				Slug:        "project-2",
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			},
		}

		mockRepo.On("List", mock.Anything).Return(projects, nil)

		req := httptest.NewRequest("GET", "/v1/projects", nil)
		w := httptest.NewRecorder()

		c, _ := gin.CreateTestContext(w)
		c.Request = req

		handler.ListProjects(c)

		assert.Equal(t, http.StatusOK, w.Code)
		
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Contains(t, response, "projects")
		
		mockRepo.AssertExpectations(t)
	})

	t.Run("database error", func(t *testing.T) {
		mockLogger := &MockLogger{}
		handler.logger = mockLogger

		mockRepo.On("List", mock.Anything).Return([]*types.Project{}, fmt.Errorf("database error"))
		mockLogger.On("Error", mock.Anything, mock.Anything, mock.Anything).Return()

		req := httptest.NewRequest("GET", "/v1/projects", nil)
		w := httptest.NewRecorder()

		c, _ := gin.CreateTestContext(w)
		c.Request = req

		handler.ListProjects(c)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		mockRepo.AssertExpectations(t)
	})
}

func TestGetProject(t *testing.T) {
	handler, mockRepo, _, _ := setupTestHandler()

	t.Run("successful get", func(t *testing.T) {
		project := &types.Project{
			ID:          "1",
			Name:        "Test Project",
			Slug:        "test-project",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		mockRepo.On("GetBySlug", mock.Anything, "test-project").Return(project, nil)

		req := httptest.NewRequest("GET", "/v1/projects/test-project", nil)
		w := httptest.NewRecorder()

		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Params = gin.Params{gin.Param{Key: "slug", Value: "test-project"}}

		handler.GetProject(c)

		assert.Equal(t, http.StatusOK, w.Code)
		mockRepo.AssertExpectations(t)
	})

	t.Run("project not found", func(t *testing.T) {
		mockLogger := &MockLogger{}
		handler.logger = mockLogger

		mockRepo.On("GetBySlug", mock.Anything, "nonexistent").Return((*types.Project)(nil), fmt.Errorf("not found"))
		mockLogger.On("Error", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()

		req := httptest.NewRequest("GET", "/v1/projects/nonexistent", nil)
		w := httptest.NewRecorder()

		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Params = gin.Params{gin.Param{Key: "slug", Value: "nonexistent"}}

		handler.GetProject(c)

		assert.Equal(t, http.StatusNotFound, w.Code)
		mockRepo.AssertExpectations(t)
	})
}

// Benchmark test for API performance
func BenchmarkCreateProject(b *testing.B) {
	handler, mockRepo, mockCache, mockLogger := setupTestHandler()

	// Setup mocks
	mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*types.Project")).Return(nil)
	mockCache.On("DelByTag", mock.Anything, "projects").Return(nil)
	mockLogger.On("Info", mock.Anything, mock.Anything, mock.Anything).Return()

	reqBody := map[string]string{
		"name":        "Benchmark Project",
		"slug":        "benchmark-project",
		"description": "A benchmark test project",
	}
	jsonBody, _ := json.Marshal(reqBody)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/v1/projects", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		c, _ := gin.CreateTestContext(w)
		c.Request = req

		handler.CreateProject(c)
	}
}