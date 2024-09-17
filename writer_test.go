package zlogsentry

import (
	"crypto/x509"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var logEventJSON = []byte(`{"level":"error","requestId":"bee07485-2485-4f64-99e1-d10165884ca7","error":"dial timeout","time":"2020-06-25T17:19:00+03:00","test":"test","message":"test message"}`)

func TestParseLogEvent(t *testing.T) {
	ts := time.Now()

	now = func() time.Time { return ts }

	w, err := New("")
	require.Nil(t, err)

	ev, ok := w.parseLogEvent(logEventJSON)
	require.True(t, ok)
	zLevel, err := w.parseLogLevel(logEventJSON)
	assert.Nil(t, err)
	ev.Level = levelsMapping[zLevel]

	assert.Equal(t, ts, ev.Timestamp)
	assert.Equal(t, sentry.LevelError, ev.Level)
	assert.Equal(t, "zerolog", ev.Logger)
	assert.Equal(t, "test message", ev.Message)

	require.Len(t, ev.Exception, 1)
	assert.Equal(t, "dial timeout", ev.Exception[0].Value)

	require.Len(t, ev.Extra, 2)
	assert.Equal(t, "test", ev.Extra["test"])
	assert.Equal(t, "bee07485-2485-4f64-99e1-d10165884ca7", ev.Extra["requestId"])
}

func TestParseLogLevel(t *testing.T) {
	w, err := New("")
	require.Nil(t, err)

	level, err := w.parseLogLevel(logEventJSON)
	require.Nil(t, err)
	assert.Equal(t, zerolog.ErrorLevel, level)
}

func TestWrite(t *testing.T) {
	slcD := []string{"apple", "peach", "pear"}
	jsontest, _ := json.Marshal(slcD)
	beforeSendCalled := false
	writer, err := New("", WithBeforeSend(func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
		assert.Equal(t, sentry.LevelError, event.Level)
		assert.Equal(t, "test message", event.Message)
		require.Len(t, event.Exception, 1)
		assert.Equal(t, "dial timeout", event.Exception[0].Value)
		assert.True(t, time.Since(event.Timestamp).Minutes() < 1)
		assert.Equal(t, "test", event.Extra["test"])
		assert.Equal(t, "[\"apple\",\"peach\",\"pear\"]", event.Extra["testjson"])
		assert.Equal(t, "bee07485-2485-4f64-99e1-d10165884ca7", event.Extra["requestId"])
		beforeSendCalled = true
		return event
	}))
	require.Nil(t, err)

	var zerologError error
	zerolog.ErrorHandler = func(err error) {
		zerologError = err
	}

	// use io.MultiWriter to enforce using the Write() method
	log := zerolog.New(io.MultiWriter(writer)).With().Timestamp().
		Str("requestId", "bee07485-2485-4f64-99e1-d10165884ca7").
		Logger()
	log.Err(errors.New("dial timeout")).
		RawJSON("testjson", jsontest).
		Str("test", "test").
		Msg("test message")

	require.Nil(t, zerologError)
	require.True(t, beforeSendCalled)
}

func TestWrite_TraceDoesNotPanic(t *testing.T) {
	beforeSendCalled := false
	writer, err := New("", WithBeforeSend(func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
		beforeSendCalled = true
		return event
	}))
	require.Nil(t, err)

	var zerologError error
	zerolog.ErrorHandler = func(err error) {
		zerologError = err
	}

	// use io.MultiWriter to enforce using the Write() method
	log := zerolog.New(io.MultiWriter(writer)).With().Timestamp().
		Str("requestId", "bee07485-2485-4f64-99e1-d10165884ca7").
		Logger()
	log.Trace().Str("test", "test").Msg("test message")

	require.Nil(t, zerologError)
	require.False(t, beforeSendCalled)
}

func TestWriteLevel(t *testing.T) {
	beforeSendCalled := false
	writer, err := New("", WithBeforeSend(func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
		assert.Equal(t, sentry.LevelError, event.Level)
		assert.Equal(t, "test message", event.Message)
		require.Len(t, event.Exception, 1)
		assert.Equal(t, "dial timeout", event.Exception[0].Value)
		assert.True(t, time.Since(event.Timestamp).Minutes() < 1)
		assert.Equal(t, "test", event.Extra["test"])
		assert.Equal(t, "bee07485-2485-4f64-99e1-d10165884ca7", event.Extra["requestId"])
		beforeSendCalled = true
		return event
	}))
	require.Nil(t, err)

	var zerologError error
	zerolog.ErrorHandler = func(err error) {
		zerologError = err
	}

	log := zerolog.New(writer).With().Timestamp().
		Str("requestId", "bee07485-2485-4f64-99e1-d10165884ca7").
		Logger()
	log.Err(errors.New("dial timeout")).
		Str("test", "test").
		Msg("test message")

	require.Nil(t, zerologError)
	require.True(t, beforeSendCalled)
}

func TestWrite_Disabled(t *testing.T) {
	beforeSendCalled := false
	writer, err := New("",
		WithLevels(zerolog.FatalLevel),
		WithBeforeSend(func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
			beforeSendCalled = true
			return event
		}))
	require.Nil(t, err)

	var zerologError error
	zerolog.ErrorHandler = func(err error) {
		zerologError = err
	}

	// use io.MultiWriter to enforce using the Write() method
	log := zerolog.New(io.MultiWriter(writer)).With().Timestamp().
		Str("requestId", "bee07485-2485-4f64-99e1-d10165884ca7").
		Logger()
	log.Err(errors.New("dial timeout")).
		Str("test", "test").
		Msg("test message")

	require.Nil(t, zerologError)
	require.False(t, beforeSendCalled)
}

func TestWriteLevel_Disabled(t *testing.T) {
	beforeSendCalled := false
	writer, err := New("",
		WithLevels(zerolog.FatalLevel),
		WithBeforeSend(func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
			beforeSendCalled = true
			return event
		}))
	require.Nil(t, err)

	var zerologError error
	zerolog.ErrorHandler = func(err error) {
		zerologError = err
	}

	log := zerolog.New(writer).With().Timestamp().
		Str("requestId", "bee07485-2485-4f64-99e1-d10165884ca7").
		Logger()
	log.Err(errors.New("dial timeout")).
		Str("test", "test").
		Msg("test message")

	require.Nil(t, zerologError)
	require.False(t, beforeSendCalled)
}

func BenchmarkParseLogEvent(b *testing.B) {
	w, err := New("")
	if err != nil {
		b.Errorf("failed to create writer: %v", err)
	}

	for i := 0; i < b.N; i++ {
		w.parseLogEvent(logEventJSON)
	}
}

func BenchmarkParseLogEvent_Disabled(b *testing.B) {
	w, err := New("", WithLevels(zerolog.FatalLevel))
	if err != nil {
		b.Errorf("failed to create writer: %v", err)
	}

	for i := 0; i < b.N; i++ {
		w.parseLogEvent(logEventJSON)
	}
}

func BenchmarkWriteLogEvent(b *testing.B) {
	w, err := New("")
	if err != nil {
		b.Errorf("failed to create writer: %v", err)
	}

	for i := 0; i < b.N; i++ {
		_, _ = w.Write(logEventJSON)
	}
}

func BenchmarkWriteLogEvent_Disabled(b *testing.B) {
	w, err := New("", WithLevels(zerolog.FatalLevel))
	if err != nil {
		b.Errorf("failed to create writer: %v", err)
	}

	for i := 0; i < b.N; i++ {
		_, _ = w.Write(logEventJSON)
	}
}

func BenchmarkWriteLogLevelEvent(b *testing.B) {
	w, err := New("")
	if err != nil {
		b.Errorf("failed to create writer: %v", err)
	}

	for i := 0; i < b.N; i++ {
		_, _ = w.WriteLevel(zerolog.ErrorLevel, logEventJSON)
	}
}

func BenchmarkWriteLogLevelEvent_Disabled(b *testing.B) {
	w, err := New("", WithLevels(zerolog.FatalLevel))
	if err != nil {
		b.Errorf("failed to create writer: %v", err)
	}

	for i := 0; i < b.N; i++ {
		_, _ = w.WriteLevel(zerolog.ErrorLevel, logEventJSON)
	}
}

func TestWrite_WithBreadcrumbs(t *testing.T) {
	writer, err := New("", WithBreadcrumbs())
	require.Nil(t, err)

	var zerologError error
	zerolog.ErrorHandler = func(err error) {
		zerologError = err
	}

	log := zerolog.New(writer).With().Timestamp().Logger()
	log.Error().Msg("test breadcrumb")

	require.Nil(t, zerologError)
	// assuming the breadcrumbs can be inspected, verify they were added
	// (sentry SDK specific testing might be tricky without a running context)
}

func TestNewWithInvalidDSN(t *testing.T) {
	_, err := New("invalid_dsn")
	assert.NotNil(t, err)
}

func TestNewWithHub(t *testing.T) {
	hub := sentry.NewHub(nil, nil)
	writer, err := NewWithHub(hub)
	require.Nil(t, err)

	assert.NotNil(t, writer)
}

func TestNewWithHub_NilHub(t *testing.T) {
	_, err := NewWithHub(nil)
	assert.NotNil(t, err)
	assert.Equal(t, "hub cannot be nil", err.Error())
}

func TestCloseSuccess(t *testing.T) {
	writer, err := New("")
	require.Nil(t, err)

	err = writer.Close()
	assert.Nil(t, err)
}

func TestWriteLevel_UnmappedLevel(t *testing.T) {
	writer, err := New("")
	require.Nil(t, err)

	var zerologError error
	zerolog.ErrorHandler = func(err error) {
		zerologError = err
	}

	data := []byte(`{"level":"unknown","message":"ignored message"}`)
	n, err := writer.WriteLevel(zerolog.NoLevel, data)

	assert.Equal(t, len(data), n)
	assert.Nil(t, err)
	assert.Nil(t, zerologError)
}

func TestWrite_InvalidJSON(t *testing.T) {
	writer, err := New("")
	require.Nil(t, err)

	var zerologError error
	zerolog.ErrorHandler = func(err error) {
		zerologError = err
	}

	invalidJSON := []byte(`{"level": error"}`)
	n, err := writer.Write(invalidJSON)

	assert.Equal(t, len(invalidJSON), n)
	assert.Nil(t, err)
	assert.Nil(t, zerologError)
}

func TestWriteLevel_InvalidLevel(t *testing.T) {
	writer, err := New("")
	require.Nil(t, err)

	n, err := writer.WriteLevel(99, logEventJSON) // 99 is an invalid level

	assert.Equal(t, len(logEventJSON), n)
	assert.Nil(t, err)
}

func TestAddBreadcrumb(t *testing.T) {
	var capturedEvent *sentry.Event

	beforeSend := func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
		// Capture the event to inspect later
		capturedEvent = event
		return event
	}

	writer, err := New("", WithBreadcrumbs(), WithBeforeSend(beforeSend))
	require.Nil(t, err)

	hub := sentry.CurrentHub().Clone()
	writer.hub = hub

	event := &sentry.Event{
		Level:   sentry.LevelError,
		Message: "Breadcrumb test",
		Extra:   map[string]interface{}{"category": "test-category"},
	}

	// Trigger addBreadcrumb by calling Write with a level that is not enabled for sending
	writer.addBreadcrumb(event)

	// Simulate capturing an event that would include breadcrumbs
	hub.CaptureMessage("trigger breadcrumbs")

	// Ensure the event was captured through beforeSend callback
	require.NotNil(t, capturedEvent)
	require.NotEmpty(t, capturedEvent.Breadcrumbs)

	// Inspect the last breadcrumb added
	lastBreadcrumb := capturedEvent.Breadcrumbs[len(capturedEvent.Breadcrumbs)-1]
	assert.Equal(t, "test-category", lastBreadcrumb.Category)
	assert.Equal(t, "Breadcrumb test", lastBreadcrumb.Message)
	assert.Equal(t, "error", lastBreadcrumb.Type)
	assert.Equal(t, event.Extra, lastBreadcrumb.Data)
}

func TestWithLevels(t *testing.T) {
	writer, err := New("", WithLevels(zerolog.DebugLevel, zerolog.WarnLevel))
	require.Nil(t, err)

	expectedLevels := map[zerolog.Level]struct{}{
		zerolog.DebugLevel: {},
		zerolog.WarnLevel:  {},
	}

	assert.Equal(t, expectedLevels, writer.levels)
}

func TestWithSampleRate(t *testing.T) {
	sampleRate := 0.5
	writer, err := New("", WithSampleRate(sampleRate))
	require.Nil(t, err)

	assert.Equal(t, sampleRate, writer.hub.Client().Options().SampleRate)
}

func TestWithRelease(t *testing.T) {
	release := "v1.0.0"
	writer, err := New("", WithRelease(release))
	require.Nil(t, err)

	assert.Equal(t, release, writer.hub.Client().Options().Release)
}

func TestWithEnvironment(t *testing.T) {
	environment := "production"
	writer, err := New("", WithEnvironment(environment))
	require.Nil(t, err)

	assert.Equal(t, environment, writer.hub.Client().Options().Environment)
}

func TestWithServerName(t *testing.T) {
	serverName := "test-server"
	writer, err := New("", WithServerName(serverName))
	require.Nil(t, err)

	assert.Equal(t, serverName, writer.hub.Client().Options().ServerName)
}

func TestWithIgnoreErrors(t *testing.T) {
	ignoreErrors := []string{"timeout", "connection refused"}
	writer, err := New("", WithIgnoreErrors(ignoreErrors))
	require.Nil(t, err)

	assert.Equal(t, ignoreErrors, writer.hub.Client().Options().IgnoreErrors)
}

func TestWithBreadcrumbs(t *testing.T) {
	writer, err := New("", WithBreadcrumbs())
	require.Nil(t, err)

	assert.True(t, writer.withBreadcrumbs)
}

func TestWithDebug(t *testing.T) {
	writer, err := New("", WithDebug())
	require.Nil(t, err)

	assert.True(t, writer.hub.Client().Options().Debug)
}

func TestWithTracing(t *testing.T) {
	writer, err := New("", WithTracing())
	require.Nil(t, err)

	assert.True(t, writer.hub.Client().Options().EnableTracing)
}

func TestWithTracingSampleRate(t *testing.T) {
	tracingSampleRate := 0.2
	writer, err := New("", WithTracingSampleRate(tracingSampleRate))
	require.Nil(t, err)

	assert.Equal(t, tracingSampleRate, writer.hub.Client().Options().TracesSampleRate)
}

func TestWithBeforeSend(t *testing.T) {
	called := false
	beforeSend := func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
		called = true
		return event
	}
	writer, err := New("", WithBeforeSend(beforeSend))
	require.Nil(t, err)

	writer.hub.CaptureMessage("test")

	assert.True(t, called)
}

func TestWithDebugWriter(t *testing.T) {
	debugWriter := &testWriter{}
	writer, err := New("", WithDebugWriter(debugWriter))
	require.Nil(t, err)

	assert.Equal(t, debugWriter, writer.hub.Client().Options().DebugWriter)
}

func TestWithHttpClient(t *testing.T) {
	httpClient := &http.Client{}
	writer, err := New("", WithHttpClient(httpClient))
	require.Nil(t, err)

	assert.Equal(t, httpClient, writer.hub.Client().Options().HTTPClient)
}

func TestWithHttpProxy(t *testing.T) {
	httpProxy := "http://proxy.example.com"
	writer, err := New("", WithHttpProxy(httpProxy))
	require.Nil(t, err)

	assert.Equal(t, httpProxy, writer.hub.Client().Options().HTTPProxy)
}

func TestWithHttpsProxy(t *testing.T) {
	httpsProxy := "https://proxy.example.com"
	writer, err := New("", WithHttpsProxy(httpsProxy))
	require.Nil(t, err)

	assert.Equal(t, httpsProxy, writer.hub.Client().Options().HTTPSProxy)
}

func TestWithCaCerts(t *testing.T) {
	caCerts := &x509.CertPool{}
	writer, err := New("", WithCaCerts(caCerts))
	require.Nil(t, err)

	assert.Equal(t, caCerts, writer.hub.Client().Options().CaCerts)
}

func TestWithMaxErrorDepth(t *testing.T) {
	maxErrorDepth := 5
	writer, err := New("", WithMaxErrorDepth(maxErrorDepth))
	require.Nil(t, err)

	assert.Equal(t, maxErrorDepth, writer.hub.Client().Options().MaxErrorDepth)
}

// Utility struct for testing WithDebugWriter
type testWriter struct{}

func (t *testWriter) Write(p []byte) (n int, err error) {
	return len(p), nil
}
