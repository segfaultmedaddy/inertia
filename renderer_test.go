package inertia

import (
	"context"
	"encoding/json"
	"errors"
	"html/template"
	"net/http"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"go.inout.gg/inertia/internal/inertiaheader"
	"go.inout.gg/inertia/internal/inertiassr"
	"go.inout.gg/inertia/internal/inertiatest"
)

//nolint:gochecknoglobals
var testTemplate = `<!DOCTYPE html>
<html>
<head>
				<title>Test Template</title>
</head>
<body>
				{{ .InertiaBody }}
</body>
</html>`

//nolint:gochecknoglobals
var testTpl = template.Must(template.New("test").Parse(testTemplate))

func testFS() fstest.MapFS {
	return fstest.MapFS{
		"templates/app.html": &fstest.MapFile{
			Data: []byte(testTemplate),
			Mode: 0o644,
		},
	}
}

func TestNew(t *testing.T) {
	t.Parallel()

	// Test cases
	tests := []struct {
		config    *Config
		tpl       *template.Template
		name      string
		wantPanic bool
	}{
		{name: "invalidate template", tpl: nil, wantPanic: true},
		{name: "empty config", tpl: testTpl},
		{name: "valid config", tpl: testTpl, config: &Config{Version: "1.0.0", RootViewID: "test-app"}},
		{name: "invalid RootViewID", tpl: testTpl, config: &Config{RootViewID: ""}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.wantPanic {
				assert.Panics(t, func() {
					New(tt.tpl, tt.config)
				}, "New should panic")

				return
			}

			renderer := New(tt.tpl, tt.config)
			assert.NotNil(t, renderer, "New should return renderer")
		})
	}
}

func TestFromFS(t *testing.T) {
	// Test cases
	tests := []struct {
		name        string
		path        string
		config      *Config
		wantVersion string
		wantErr     bool
		wantPanic   bool
	}{
		{
			name:        "valid template with config",
			path:        "templates/*.html",
			config:      &Config{Version: "1.0.0", RootViewID: "test-app"},
			wantVersion: "1.0.0",
			wantErr:     false,
			wantPanic:   false,
		},
		{
			name:        "valid template without config",
			path:        "templates/*.html",
			config:      nil,
			wantVersion: "",
			wantErr:     false,
			wantPanic:   false,
		},
		{
			name:      "invalid template path",
			path:      "nonexistent/*.html",
			config:    nil,
			wantErr:   true,
			wantPanic: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name+" (FromFS)", func(t *testing.T) {
			renderer, err := FromFS(testFS(), tt.path, tt.config)

			if tt.wantErr {
				require.Error(t, err, "FromFS should return error with invalid template path")
				assert.Nil(t, renderer, "renderer should be nil when error occurs")

				return
			}

			require.NoError(t, err, "FromFS should not return error with valid template path")
			assert.NotNil(t, renderer, "renderer should not be nil")
			assert.Equal(t, tt.wantVersion, renderer.Version(), "renderer version should match config")
		})

		t.Run(tt.name+" (MustFromFS)", func(t *testing.T) {
			if tt.wantPanic {
				assert.Panics(t, func() {
					MustFromFS(testFS(), tt.path, tt.config)
				}, "FromFS should panic")

				return
			}

			var renderer *Renderer

			assert.NotPanics(t, func() {
				renderer = MustFromFS(testFS(), tt.path, tt.config)
			}, "FromFS should not panic with valid template path")

			assert.NotNil(t, renderer, "renderer should not be nil")
			assert.Equal(t, tt.wantVersion, renderer.Version(), "renderer version should match config")
		})
	}
}

func TestRenderer_Render(t *testing.T) {
	t.Parallel()

	// Basic template for testing
	basicTemplate := `<!DOCTYPE html>
<html>
<head>
	<title>Test Template</title>
	{{.InertiaHead}}
</head>
<body>
	{{.InertiaBody}}
</body>
</html>`

	basicTpl := template.Must(template.New("test").Parse(basicTemplate))

	// Create a mock SSR client
	ctrl := gomock.NewController(t)

	t.Cleanup(func() {
		ctrl.Finish()
	})

	mockSsrClient := inertiassr.NewMockSsrClient(ctrl)
	mockSsrClient.EXPECT().Render(gomock.Any(), gomock.Any()).Return(&inertiassr.SsrTemplateData{
		Head: "<title>SSR Title</title>",
		Body: "<div>SSR Content</div>",
	}, nil).AnyTimes()

	errorMockSsrClient := inertiassr.NewMockSsrClient(ctrl)
	errorMockSsrClient.EXPECT().Render(gomock.Any(), gomock.Any()).Return(nil, errors.New("SSR error")).AnyTimes()

	// Define a validation function type
	type responseValidator func(t *testing.T, body []byte)

	// Test cases for the Render method
	tests := []struct {
		renderer             *Renderer
		reqConfig            *inertiatest.RequestConfig
		expectedHeaders      map[string]string
		validateResponse     responseValidator
		name                 string
		componentName        string
		options              []Option
		expectedBodyContains []string
		expectedStatusCode   int
		expectJSON           bool
		expectError          bool
	}{
		{
			name: "non-inertia request - html response",
			renderer: New(basicTpl, &Config{
				Version:    "1.0.0",
				RootViewID: "app",
			}),
			reqConfig:          &inertiatest.RequestConfig{},
			componentName:      "TestComponent",
			options:            []Option{},
			expectedStatusCode: http.StatusOK,
			expectedHeaders: map[string]string{
				inertiaheader.HeaderContentType: inertiaheader.ContentTypeHTML,
			},
			expectJSON:  false,
			expectError: false,
			validateResponse: func(t *testing.T, body []byte) {
				t.Helper()

				bodyStr := string(body)
				assert.Contains(t, bodyStr, `<div id="app" data-page="`)
				assert.Contains(t, bodyStr, template.HTMLEscapeString(`"component":"TestComponent"`))
				assert.Contains(t, bodyStr, template.HTMLEscapeString(`"version":"1.0.0"`))
			},
		},
		{
			name: "inertia request - json response",
			renderer: New(basicTpl, &Config{
				Version:    "1.0.0",
				RootViewID: "app",
			}),
			reqConfig: &inertiatest.RequestConfig{
				Inertia: true,
			},
			componentName:      "TestComponent",
			options:            []Option{},
			expectedStatusCode: http.StatusOK,
			expectedHeaders: map[string]string{
				inertiaheader.HeaderContentType: inertiaheader.ContentTypeJSON,
				inertiaheader.HeaderXInertia:    "true",
			},
			expectJSON:  true,
			expectError: false,
			validateResponse: func(t *testing.T, body []byte) {
				t.Helper()

				var page map[string]any

				err := json.Unmarshal(body, &page)
				require.NoError(t, err, "failed to parse JSON response")

				assert.Equal(t, "TestComponent", page["component"])
				assert.Equal(t, "1.0.0", page["version"])
			},
		},
		{
			name: "ssr enabled - html response",
			renderer: New(basicTpl, &Config{
				Version:    "1.0.0",
				RootViewID: "app",
				SsrClient:  mockSsrClient,
			}),
			reqConfig:          &inertiatest.RequestConfig{},
			componentName:      "TestComponent",
			options:            []Option{},
			expectedStatusCode: http.StatusOK,
			expectedHeaders: map[string]string{
				inertiaheader.HeaderContentType: inertiaheader.ContentTypeHTML,
			},
			expectJSON:  false,
			expectError: false,
			validateResponse: func(t *testing.T, body []byte) {
				t.Helper()

				bodyStr := string(body)
				assert.Contains(t, bodyStr, "<title>SSR Title</title>")
				assert.Contains(t, bodyStr, "<div>SSR Content</div>")
			},
		},
		{
			name: "ssr with error - returns error",
			renderer: New(basicTpl, &Config{
				Version:   "1.0.0",
				SsrClient: errorMockSsrClient,
			}),
			reqConfig:     &inertiatest.RequestConfig{},
			componentName: "TestComponent",
			options:       []Option{},
			expectError:   true,
			validateResponse: func(t *testing.T, _ []byte) {
				t.Helper()

				// No validation needed as we expect an error
			},
		},
		{
			name: "with root view attributes",
			renderer: New(basicTpl, &Config{
				Version:    "1.0.0",
				RootViewID: "app",
				RootViewAttrs: map[string]string{
					"class":     "container",
					"data-test": "value",
					"data-page": "should-be-skipped", // Should be ignored
				},
			}),
			reqConfig:          &inertiatest.RequestConfig{},
			componentName:      "TestComponent",
			options:            []Option{},
			expectedStatusCode: http.StatusOK,
			expectJSON:         false,
			expectError:        false,
			validateResponse: func(t *testing.T, body []byte) {
				t.Helper()

				bodyStr := string(body)
				assert.Contains(t, bodyStr, `<div id="app" data-page="`)
				assert.Contains(t, bodyStr, `class="container"`)
				assert.Contains(t, bodyStr, `data-test="value"`)
			},
		},
		{
			name: "with validation errors",
			renderer: New(basicTpl, &Config{
				Version:    "1.0.0",
				RootViewID: "app",
			}),
			reqConfig:     &inertiatest.RequestConfig{Inertia: true},
			componentName: "TestComponent",
			options: []Option{
				WithValidationErrors(ValidationErrors{
					NewValidationError("name", "Name is required"),
					NewValidationError("email", "Invalid email"),
				}, DefaultErrorBag),
			},
			expectedStatusCode: http.StatusOK,
			expectJSON:         false,
			expectError:        false,
			validateResponse: func(t *testing.T, body []byte) {
				t.Helper()

				var page map[string]any

				err := json.Unmarshal(body, &page)
				require.NoError(t, err, "Failed to parse response JSON")

				props, ok := page["props"].(map[string]any)
				require.True(t, ok, "props not found")

				errors, ok := props["errors"].(map[string]any)
				require.True(t, ok, "errors not found")

				assert.Equal(t, "Name is required", errors["name"], "name error doesn't match")
				assert.Equal(t, "Invalid email", errors["email"], "email error doesn't match")
			},
		},
		{
			name: "with custom error bag",
			renderer: New(basicTpl, &Config{
				Version:    "1.0.0",
				RootViewID: "app",
			}),
			reqConfig: &inertiatest.RequestConfig{
				Inertia: true,
			},
			componentName: "TestComponent",
			options: []Option{
				WithValidationErrors(ValidationErrors{
					NewValidationError("name", "Name is required"),
				}, "custom_errors"),
			},
			expectedStatusCode: http.StatusOK,
			expectJSON:         false,
			expectError:        false,
			validateResponse: func(t *testing.T, body []byte) {
				t.Helper()

				var page map[string]any

				err := json.Unmarshal(body, &page)
				require.NoError(t, err, "Failed to parse response JSON")

				props, ok := page["props"].(map[string]any)
				require.True(t, ok, "props not found")

				customErrors, ok := props["custom_errors"].(map[string]any)
				require.True(t, ok, "custom_errors not found")

				errors, ok := customErrors["errors"].(map[string]any)
				require.True(t, ok, "errors not found")

				assert.Equal(t, "Name is required", errors["name"], "name error doesn't match")
			},
		},
		{
			name: "with partial component request",
			renderer: New(basicTpl, &Config{
				Version:    "1.0.0",
				RootViewID: "app",
			}),
			reqConfig: &inertiatest.RequestConfig{
				Inertia:          true,
				PartialComponent: "TestComponent",
				Whitelist:        []string{"title", "content"},
			},
			componentName: "TestComponent",
			options: []Option{
				WithProps(Props{
					NewProp("title", "Test Title", nil),
					NewProp("content", "Test Content", nil),
					NewProp("hidden", "Should Not Be Included", nil),
				}),
			},
			expectedStatusCode: http.StatusOK,
			expectedHeaders: map[string]string{
				inertiaheader.HeaderContentType: inertiaheader.ContentTypeJSON,
				inertiaheader.HeaderXInertia:    "true",
			},
			expectJSON:  true,
			expectError: false,
			validateResponse: func(t *testing.T, body []byte) {
				t.Helper()

				var page map[string]any

				err := json.Unmarshal(body, &page)
				require.NoError(t, err, "failed to parse JSON response")

				assert.Equal(t, "TestComponent", page["component"])

				props, ok := page["props"].(map[string]any)
				require.True(t, ok, "props should be a map")

				assert.Contains(t, props, "title", "title prop should be included")
				assert.Contains(t, props, "content", "content prop should be included")
				assert.NotContains(t, props, "hidden", "hidden prop should not be included")
			},
		},
		{
			name: "with partial component request with blacklist",
			renderer: New(basicTpl, &Config{
				Version:    "1.0.0",
				RootViewID: "app",
			}),
			reqConfig: &inertiatest.RequestConfig{
				Inertia:          true,
				PartialComponent: "TestComponent",
				Blacklist:        []string{"hidden"},
			},
			componentName: "TestComponent",
			options: []Option{
				WithProps(Props{
					NewProp("title", "Test Title", nil),
					NewProp("content", "Test Content", nil),
					NewProp("hidden", "Should Not Be Included", nil),
				}),
			},
			expectedStatusCode: http.StatusOK,
			expectJSON:         true,
			expectError:        false,
			validateResponse: func(t *testing.T, body []byte) {
				t.Helper()

				var page map[string]any

				err := json.Unmarshal(body, &page)
				require.NoError(t, err, "failed to parse JSON response")

				// Check props are correctly filtered
				props, ok := page["props"].(map[string]any)
				require.True(t, ok, "props should be a map[string]any")

				assert.Contains(t, props, "title", "title prop should be included")
				assert.Contains(t, props, "content", "content prop should be included")
				assert.NotContains(t, props, "hidden", "hidden prop should not be included")
			},
		},
		{
			name: "with lazy props",
			renderer: New(basicTpl, &Config{
				Version:    "1.0.0",
				RootViewID: "app",
			}),
			reqConfig:     &inertiatest.RequestConfig{Inertia: true},
			componentName: "TestComponent",
			options: []Option{
				WithProps(Props{
					NewProp("visible", "Visible Content", nil),
					NewDeferred(
						"lazy",
						LazyFunc(
							func(context.Context) (any, error) { return "Lazy Content", nil },
						),
						&DeferredOptions{
							Group: "group1",
						},
					),
				}),
			},
			expectedStatusCode: http.StatusOK,
			expectJSON:         false,
			expectError:        false,
			validateResponse: func(t *testing.T, body []byte) {
				t.Helper()

				var page map[string]any

				err := json.Unmarshal(body, &page)
				require.NoError(t, err, "Failed to parse response JSON")

				// Check props
				props, ok := page["props"].(map[string]any)
				require.True(t, ok, "props not found")
				assert.Equal(t, "Visible Content", props["visible"], "visible prop doesn't match")

				// Check deferred props
				deferredProps, ok := page["deferredProps"].(map[string]any)
				require.True(t, ok, "deferredProps not found")

				group1, ok := deferredProps["group1"].([]any)
				require.True(t, ok, "group1 not found in deferredProps")

				assert.Contains(t, group1, "lazy", "lazy not found in group1")
			},
		},
		{
			name: "with mergeable props",
			renderer: New(basicTpl, &Config{
				Version:    "1.0.0",
				RootViewID: "app",
			}),
			reqConfig:     &inertiatest.RequestConfig{Inertia: true},
			componentName: "TestComponent",
			options: []Option{
				WithProps(Props{
					NewProp("normalProp", "Normal Value", nil),
					NewProp("mergeProp", map[string]string{"key": "value"}, &PropOptions{
						Merge: true,
					}),
				}),
			},
			expectedStatusCode: http.StatusOK,
			expectJSON:         false,
			expectError:        false,
			validateResponse: func(t *testing.T, body []byte) {
				t.Helper()

				var page map[string]any

				err := json.Unmarshal(body, &page)
				require.NoError(t, err, "Failed to parse response JSON")

				// Check merge props
				mergeProps, ok := page["mergeProps"].([]any)
				require.True(t, ok, "mergeProps not found")

				assert.Contains(t, mergeProps, "mergeProp", "mergeProp not found in mergeProps")

				// Check props
				props, ok := page["props"].(map[string]any)
				require.True(t, ok, "props not found")
				assert.Equal(t, "Normal Value", props["normalProp"], "normalProp doesn't match")

				mergeProp, ok := props["mergeProp"].(map[string]any)
				require.True(t, ok, "mergeProp not found or not a map")
				assert.Equal(t, "value", mergeProp["key"], "mergeProp.key doesn't match")
			},
		},
		{
			name: "with merge props with reset",
			renderer: New(basicTpl, &Config{
				Version:    "1.0.0",
				RootViewID: "app",
			}),
			reqConfig: &inertiatest.RequestConfig{
				Inertia:    true,
				ResetProps: []string{"mergeProp"},
			},
			componentName: "TestComponent",
			options: []Option{
				WithProps(Props{
					NewProp("mergeProp", map[string]string{"key": "value"}, &PropOptions{
						Merge: true,
					}),
				}),
			},
			expectedStatusCode: http.StatusOK,
			expectJSON:         false,
			expectError:        false,
			validateResponse: func(t *testing.T, body []byte) {
				t.Helper()

				// Just check that the response is valid JSON - the blacklisted prop
				// should be excluded from merge
				var responseObj map[string]any

				err := json.Unmarshal(body, &responseObj)
				require.NoError(t, err, "Failed to parse response JSON")

				_, ok := responseObj["mergeProps"]
				require.False(t, ok, "mergeProps should not be found")
			},
		},
		{
			name: "clear history flag",
			renderer: New(basicTpl, &Config{
				Version:    "1.0.0",
				RootViewID: "app",
			}),
			reqConfig:          &inertiatest.RequestConfig{Inertia: true},
			componentName:      "TestComponent",
			options:            []Option{WithClearHistory()},
			expectedStatusCode: http.StatusOK,
			expectJSON:         false,
			expectError:        false,
			validateResponse: func(t *testing.T, body []byte) {
				t.Helper()

				var page map[string]any

				err := json.Unmarshal(body, &page)
				require.NoError(t, err, "Failed to parse response JSON")

				clearHistory, ok := page["clearHistory"].(bool)
				require.True(t, ok, "clearHistory not found or not a boolean")
				assert.True(t, clearHistory, "clearHistory should be true")
			},
		},
		{
			name: "encrypt history flag",
			renderer: New(basicTpl, &Config{
				Version:    "1.0.0",
				RootViewID: "app",
			}),
			reqConfig:          &inertiatest.RequestConfig{Inertia: true},
			componentName:      "TestComponent",
			options:            []Option{WithEncryptHistory()},
			expectedStatusCode: http.StatusOK,
			expectJSON:         false,
			expectError:        false,
			validateResponse: func(t *testing.T, body []byte) {
				t.Helper()

				var page map[string]any

				err := json.Unmarshal(body, &page)
				require.NoError(t, err, "Failed to parse response JSON")

				encryptHistory, ok := page["encryptHistory"].(bool)
				require.True(t, ok, "encryptHistory not found or not a boolean")
				assert.True(t, encryptHistory, "encryptHistory should be true")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create request and recorder using inertiatest
			req, w := inertiatest.NewRequest(http.MethodGet, "/", tt.reqConfig)

			// Create a RenderContext from the options
			rCtx := RenderContext{}
			for _, opt := range tt.options {
				opt(&rCtx)
			}

			// Call the Render function
			err := tt.renderer.Render(w, req, tt.componentName, rCtx)

			// Check for expected error conditions
			if tt.expectError {
				assert.Error(t, err, "expected an error but got none")
				return
			}

			require.NoError(t, err, "unexpected error")

			// Check status code
			if tt.expectedStatusCode > 0 {
				assert.Equal(t, tt.expectedStatusCode, w.Code, "status code does not match")
			}

			// Check headers
			for key, value := range tt.expectedHeaders {
				assert.Equal(t, value, w.Header().Get(key), "header %s does not match", key)
			}

			// Run the custom validation function for this test case
			if tt.validateResponse != nil {
				tt.validateResponse(t, w.Body.Bytes())
			}

			// Keep backward compatibility check for non-JSON responses
			if !tt.expectJSON && len(tt.expectedBodyContains) > 0 {
				responseBody := w.Body.String()
				for _, expected := range tt.expectedBodyContains {
					assert.Contains(t, responseBody, expected,
						"body does not contain expected content")
				}
			}
		})
	}
}

func TestLocation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		reqConfig      *inertiatest.RequestConfig
		expectedHeader map[string]string
		name           string
		url            string
		expectedStatus int
	}{
		{
			name:           "non-inertia request",
			reqConfig:      &inertiatest.RequestConfig{},
			url:            "/redirect",
			expectedStatus: http.StatusFound, // 302 Found
			expectedHeader: map[string]string{
				"Location": "/redirect",
			},
		},
		{
			name: "inertia request",
			reqConfig: &inertiatest.RequestConfig{
				Inertia: true,
			},
			url:            "/redirect",
			expectedStatus: http.StatusConflict, // 409 Conflict
			expectedHeader: map[string]string{
				inertiaheader.HeaderXInertiaLocation: "/redirect",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req, w := inertiatest.NewRequest(http.MethodGet, "/current", tt.reqConfig)

			// Add test-specific header to test cleanup (for inertia request headers are cleaned test)
			if tt.name == "inertia request headers are cleaned" {
				req.Header.Set("Vary", "some-value")
			}

			Location(w, req, tt.url)

			assert.Equal(t, tt.expectedStatus, w.Code, "unexpected status code")

			for header, value := range tt.expectedHeader {
				assert.Equal(t, value, w.Header().Get(header),
					"unexpected header value for %s", header)
			}
		})
	}
}

func TestRenderer_Version(t *testing.T) {
	t.Parallel()

	renderer := New(testTpl, &Config{Version: "1.0.0"})
	assert.Equal(t, "1.0.0", renderer.Version(), "renderer version should match config")
}

func TestExtractHeaderValueList(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		header   string
		expected []string
	}{
		{
			name:     "empty header",
			header:   "",
			expected: nil,
		},
		{
			name:     "single value",
			header:   "test",
			expected: []string{"test"},
		},
		{
			name:     "multiple values",
			header:   "test1,test2,test3",
			expected: []string{"test1", "test2", "test3"},
		},
		{
			name:     "values with whitespace",
			header:   " test1 , test2 , test3 ",
			expected: []string{"test1", "test2", "test3"},
		},
		{
			name:     "values with mixed whitespace",
			header:   "test1,  test2,test3  ",
			expected: []string{"test1", "test2", "test3"},
		},
		{
			name:     "values with dots",
			header:   "user.name,user.email,user.age",
			expected: []string{"user.name", "user.email", "user.age"},
		},
		{
			name:     "single value with whitespace",
			header:   " test ",
			expected: []string{"test"},
		},
		{
			name:     "empty values between commas",
			header:   "test1,,test2",
			expected: []string{"test1", "", "test2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := extractHeaderValueList(tt.header)
			assert.Equal(t, tt.expected, result, "extracted list should match expected values")
		})
	}
}
