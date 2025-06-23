package gonoleks

import (
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/valyala/fasthttp"
)

// Custom log level for HTTP requests
const HTTPLevel = log.Level(1)

var (
	// Status code styles
	statusInfoStyle      = lipgloss.NewStyle().Background(lipgloss.Color("63")).Bold(true)  // 1xx
	statusSuccessStyle   = lipgloss.NewStyle().Background(lipgloss.Color("86")).Bold(true)  // 2xx
	statusRedirectStyle  = lipgloss.NewStyle().Background(lipgloss.Color("216")).Bold(true) // 3xx
	statusClientErrStyle = lipgloss.NewStyle().Background(lipgloss.Color("192")).Bold(true) // 4xx
	statusServerErrStyle = lipgloss.NewStyle().Background(lipgloss.Color("204")).Bold(true) // 5xx

	// HTTP method styles
	methodGetStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Bold(true)
	methodPostStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("192")).Bold(true)
	methodPutStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("63")).Bold(true)
	methodPatchStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("134")).Bold(true)
	methodDeleteStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("204")).Bold(true)
	methodDefaultStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("219")).Bold(true)
)

// getStatusStyle returns the appropriate pre-created style for the status code
// Colors are grouped by status code ranges to provide visual indication of response types:
// - 1xx: Informational (RoyalBlue)
// - 2xx: Success (Aquamarine)
// - 3xx: Redirection (LightSalmon)
// - 4xx: Client Error (Mindaro)
// - 5xx: Server Error (IndianRed)
func getStatusStyle(status int) lipgloss.Style {
	switch {
	case status >= StatusContinue && status < StatusOK:
		return statusInfoStyle
	case status >= StatusOK && status < StatusMultipleChoices:
		return statusSuccessStyle
	case status >= StatusMultipleChoices && status < StatusBadRequest:
		return statusRedirectStyle
	case status >= StatusBadRequest && status < StatusInternalServerError:
		return statusClientErrStyle
	default:
		return statusServerErrStyle
	}
}

// getMethodStyle returns the appropriate pre-created style for the HTTP method
func getMethodStyle(method []byte) lipgloss.Style {
	// Check common HTTP methods
	switch len(method) {
	case 3: // GET, PUT
		if method[0] == 'G' && method[1] == 'E' && method[2] == 'T' {
			return methodGetStyle
		}
		if method[0] == 'P' && method[1] == 'U' && method[2] == 'T' {
			return methodPutStyle
		}
	case 4: // POST, HEAD
		if method[0] == 'P' && method[1] == 'O' && method[2] == 'S' && method[3] == 'T' {
			return methodPostStyle
		}
		if method[0] == 'H' && method[1] == 'E' && method[2] == 'A' && method[3] == 'D' {
			return methodGetStyle // Same style as GET
		}
	case 5: // PATCH
		if method[0] == 'P' && method[1] == 'A' && method[2] == 'T' && method[3] == 'C' && method[4] == 'H' {
			return methodPatchStyle
		}
	case 6: // DELETE
		if method[0] == 'D' && method[1] == 'E' && method[2] == 'L' && method[3] == 'E' && method[4] == 'T' && method[5] == 'E' {
			return methodDeleteStyle
		}
	}

	return methodDefaultStyle
}

// setLoggerSettings configures the global logger with custom styles and output settings
// It initializes color schemes, output destination, and default logging behavior
func setLoggerSettings(settings *Settings) {
	styles := log.DefaultStyles()
	styles.Timestamp = lipgloss.NewStyle().Faint(true)
	styles.Values["error"] = lipgloss.NewStyle().Foreground(lipgloss.Color("204"))
	styles.Levels[HTTPLevel] = lipgloss.NewStyle().Foreground(lipgloss.Color("63")).Bold(true).SetString("HTTP")

	log.SetStyles(styles)
	log.SetOutput(os.Stderr)

	// Apply custom time format if provided, otherwise use default format
	if settings.LogTimeFormat == "" {
		settings.LogTimeFormat = "2006/01/02 15:04:05"
	}
	log.SetTimeFormat(settings.LogTimeFormat)

	// Setup logger prefix if provided
	if settings.LogPrefix != "" {
		log.SetPrefix(settings.LogPrefix)
	}

	// Configure caller reporting based on settings or use default
	if settings.LogReportCaller {
		log.SetReportCaller(true)
	}
}

// logWithOptionalFields logs a message with optional fields at the specified log level
// It optimizes logging by avoiding unnecessary allocations
func logWithOptionalFields(level log.Level, format string, args []any, fields []any) {
	if len(fields) == 0 {
		log.Logf(level, format, args...)
		return
	}

	// Use a pre-allocated logger with fields
	logger := log.With(fields...)
	logger.Logf(level, format, args...)
}

// logHTTPTransaction records details of an HTTP request-response cycle with color-coded formatting
// It logs the status code, latency, HTTP method and path, with optional response body or error message
func logHTTPTransaction(ctx *fasthttp.RequestCtx, latency time.Duration) {
	settings, _ := ctx.UserValue("gonoleksSettings").(*Settings)
	status := ctx.Response.StatusCode()
	method := ctx.Method()

	// Pre-allocate logFields with a reasonable capacity to avoid reallocations
	var logFields []any

	// Only convert body to string if we're going to use it
	if settings != nil && ((status >= StatusBadRequest && settings.LogReportResponseError) ||
		(status < StatusBadRequest && settings.LogReportResponseBody)) {
		body := getString(ctx.Response.Body())

		if status >= StatusBadRequest {
			if settings.LogReportResponseError && len(body) > 0 {
				logFields = append(logFields, "error", body)
			}
		} else if settings.LogReportResponseBody && len(body) > 0 {
			logFields = append(logFields, "responseBody", body)
		}
	}

	// Add request headers if configured
	if settings != nil && settings.LogReportRequestHeaders {
		// Create a map for headers
		headers := make(map[string]string, 10) // Provide initial capacity
		ctx.Request.Header.VisitAll(func(key, value []byte) {
			headers[getString(key)] = getString(value)
		})
		if len(headers) > 0 {
			logFields = append(logFields, "headers", headers)
		}
	}

	// Add request body if configured
	if settings != nil && settings.LogReportRequestBody {
		reqBody := getString(ctx.Request.Body())
		if len(reqBody) > 0 {
			logFields = append(logFields, "requestBody", reqBody)
		}
	}

	// Add client IP if configured
	if settings != nil && settings.LogReportIP {
		ip := ctx.RemoteIP().String()
		if len(ip) > 0 {
			logFields = append(logFields, "ip", ip)
		}
	}

	// Add host if configured
	if settings != nil && settings.LogReportHost {
		host := getString(ctx.Host())
		if len(host) > 0 {
			logFields = append(logFields, "host", host)
		}
	}

	// Add user agent if configured
	if settings != nil && settings.LogReportUserAgent {
		userAgent := getString(ctx.Request.Header.UserAgent())
		if len(userAgent) > 0 {
			logFields = append(logFields, "userAgent", userAgent)
		}
	}

	format := "%s| %9s | %s %q"
	args := []any{
		getStatusStyle(status).Width(5).Align(lipgloss.Center).Render(fmt.Sprint(status)),
		latency,
		getMethodStyle(method).Render(fmt.Sprintf("%-7s", method)),
		ctx.Path(),
	}
	logWithOptionalFields(HTTPLevel, format, args, logFields)
}
