package gonoleks

import (
	"fmt"
	"io"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/muesli/termenv"
	"github.com/valyala/fasthttp"
)

// LogFormatterParams contains the parameters for custom log formatting
type LogFormatterParams struct {
	// Request contains the HTTP request
	Request *fasthttp.Request

	// TimeStamp shows the time after the server returns a response (auto-styled with faint when formatted)
	TimeStamp time.Time

	// StatusCode is HTTP response code
	StatusCode int

	// Latency is how much time the server cost to process a certain request (auto-styled with faint)
	Latency time.Duration

	// ClientIP equals Context.ClientIP()
	ClientIP string

	// Method is the HTTP method given to the request
	Method string

	// Path is a path the client requests
	Path string

	// ErrorMessage is set if error has occurred in processing the request
	ErrorMessage string

	// BodySize is the size of the Response Body
	BodySize int

	// Keys are the keys set on the request's context
	Keys map[string]any
}

// LoggerConfig defines the config for Logger middleware
type LoggerConfig struct {
	// Formatter is the log format function
	Formatter LogFormatter // Default = DefaultLogFormatter

	// Output is a writer where logs are written
	Output io.Writer // Default = os.Stdout

	// SkipPaths is an url path array which logs are not written
	SkipPaths []string
}

// LogFormatter gives the signature of the formatter function passed to LoggerWithFormatter
type LogFormatter func(params LogFormatterParams) string

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

// DefaultLogFormatter is the default log format function Logger middleware uses
var DefaultLogFormatter = func(param LogFormatterParams) string {
	styledStatus := getStatusStyle(param.StatusCode).Width(5).Align(lipgloss.Center).Render(fmt.Sprint(param.StatusCode))
	styledMethod := getMethodStyle(param.Method).Render(fmt.Sprintf("%-7s", param.Method))

	return fmt.Sprintf("%s| %13v | %15s | %s %q",
		styledStatus,
		param.Latency,
		param.ClientIP,
		styledMethod,
		param.Path,
	)
}

// DisableConsoleColor disables color output in the console
func DisableConsoleColor() {
	p := termenv.Ascii
	lipgloss.SetColorProfile(p)
	log.SetColorProfile(p)
}

// ForceConsoleColor forces color output in the console
func ForceConsoleColor() {
	p := termenv.TrueColor
	lipgloss.SetColorProfile(p)
	log.SetColorProfile(p)
}

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
func getMethodStyle(method string) lipgloss.Style {
	// Use more efficient byte comparison with constants
	switch {
	case len(method) == 3 && method[0] == 'G' && method[1] == 'E' && method[2] == 'T':
		return methodGetStyle
	case len(method) == 3 && method[0] == 'P' && method[1] == 'U' && method[2] == 'T':
		return methodPutStyle
	case len(method) == 4 && method[0] == 'P' && method[1] == 'O' && method[2] == 'S' && method[3] == 'T':
		return methodPostStyle
	case len(method) == 4 && method[0] == 'H' && method[1] == 'E' && method[2] == 'A' && method[3] == 'D':
		return methodGetStyle
	case len(method) == 5 && method[0] == 'P' && method[1] == 'A' && method[2] == 'T' && method[3] == 'C' && method[4] == 'H':
		return methodPatchStyle
	case len(method) == 6 && method[0] == 'D' && method[1] == 'E' && method[2] == 'L' && method[3] == 'E' && method[4] == 'T' && method[5] == 'E':
		return methodDeleteStyle
	default:
		return methodDefaultStyle
	}
}

// Logger instances a Logger middleware that will write the logs to os.Stdout
// By default, Logger() will output logs to os.Stdout
func Logger() handlerFunc {
	return LoggerWithConfig(LoggerConfig{})
}

// LoggerWithFormatter instance a Logger middleware with the specified log format function
func LoggerWithFormatter(f LogFormatter) handlerFunc {
	return LoggerWithConfig(LoggerConfig{
		Formatter: f,
	})
}

// LoggerWithWriter instance a Logger middleware with the specified writer buffer
// Example: os.Stdout, a file opened in write mode, a socket...
func LoggerWithWriter(out io.Writer, notlogged ...string) handlerFunc {
	log.SetOutput(out)

	return LoggerWithConfig(LoggerConfig{
		Output:    out,
		SkipPaths: notlogged,
	})
}

// LoggerWithConfig instance a Logger middleware with config
func LoggerWithConfig(conf LoggerConfig) handlerFunc {
	formatter := conf.Formatter
	if formatter == nil {
		formatter = DefaultLogFormatter
	}

	// Check if using DefaultLogFormatter
	usingDefaultLogFormatter := formatter == nil || fmt.Sprintf("%p", formatter) == fmt.Sprintf("%p", DefaultLogFormatter)

	notlogged := conf.SkipPaths

	var skip map[string]struct{}

	if length := len(notlogged); length > 0 {
		skip = make(map[string]struct{}, length)

		for _, path := range notlogged {
			skip[path] = struct{}{}
		}
	}

	return func(c *Context) {
		// Start timer
		start := time.Now()
		// Avoid string conversion - use byte slices directly
		path := c.requestCtx.Path()      // Already []byte, no need to convert
		raw := c.requestCtx.RequestURI() // Already []byte

		// Process request
		c.Next()

		// Log only when path is not being skipped
		// Convert to string only for map lookup
		pathStr := string(path)
		if _, ok := skip[pathStr]; !ok {
			param := LogFormatterParams{
				Request:      &c.requestCtx.Request,
				TimeStamp:    time.Now(),
				Latency:      time.Since(start),
				ClientIP:     c.ClientIP(),
				Method:       string(c.requestCtx.Method()),
				StatusCode:   c.requestCtx.Response.StatusCode(),
				ErrorMessage: "",
				BodySize:     len(c.requestCtx.Response.Body()),
				Keys:         nil,
			}

			// Set path - avoid redundant string conversion
			if len(raw) > 0 {
				param.Path = pathStr
			}

			// Extract error message if any - avoid string conversion unless needed
			if c.requestCtx.Response.StatusCode() >= StatusBadRequest {
				body := c.requestCtx.Response.Body()
				if len(body) > 0 {
					// Only convert to string when actually needed
					param.ErrorMessage = string(body)
				}
			}

			// Extract keys from context if available
			if keys := c.requestCtx.UserValue("keys"); keys != nil {
				if keyMap, ok := keys.(map[string]any); ok {
					param.Keys = keyMap
				}
			}

			logMessage := formatter(param)

			if usingDefaultLogFormatter {
				// Use Debug log level with timestamp for DefaultLogFormatter
				log.SetReportTimestamp(true)
				log.SetLevel(log.DebugLevel)
				log.Debugf(logMessage)
			} else {
				// Use regular Printf without log level and timestamp for custom formatters
				log.SetReportTimestamp(false)
				log.Printf(logMessage)
			}
		}
	}
}
