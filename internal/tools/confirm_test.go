package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// plainSession is a client session without elicitation support.
type plainSession struct{}

func (plainSession) Initialize()       {}
func (plainSession) Initialized() bool { return true }
func (plainSession) NotificationChannel() chan<- mcp.JSONRPCNotification {
	return make(chan mcp.JSONRPCNotification, 1)
}
func (plainSession) SessionID() string { return "test-session" }

// elicitSession is a client that advertised the elicitation capability and
// answers requests with a canned result.
type elicitSession struct {
	plainSession
	calls  int
	result *mcp.ElicitationResult
}

func (s *elicitSession) RequestElicitation(context.Context, mcp.ElicitationRequest) (*mcp.ElicitationResult, error) {
	s.calls++
	return s.result, nil
}

func (s *elicitSession) GetClientInfo() mcp.Implementation            { return mcp.Implementation{Name: "test"} }
func (s *elicitSession) SetClientInfo(mcp.Implementation)             {}
func (s *elicitSession) SetClientCapabilities(mcp.ClientCapabilities) {}
func (s *elicitSession) GetClientCapabilities() mcp.ClientCapabilities {
	return mcp.ClientCapabilities{Elicitation: &mcp.ElicitationCapability{}}
}

func accepted(content any) *mcp.ElicitationResult {
	return &mcp.ElicitationResult{
		ElicitationResponse: mcp.ElicitationResponse{
			Action:  mcp.ElicitationResponseActionAccept,
			Content: content,
		},
	}
}

func TestConfirmWrite(t *testing.T) {
	tests := []struct {
		name    string
		result  *mcp.ElicitationResult
		allowed bool
	}{
		{"confirmed", accepted(map[string]any{"confirm": true}), true},
		{"confirmed as string", accepted(map[string]any{"confirm": "true"}), true},
		{"refused in form", accepted(map[string]any{"confirm": false}), false},
		{"accepted without content", accepted(nil), false},
		{"missing field", accepted(map[string]any{}), false},
		{"declined", &mcp.ElicitationResult{
			ElicitationResponse: mcp.ElicitationResponse{Action: mcp.ElicitationResponseActionDecline},
		}, false},
		{"cancelled", &mcp.ElicitationResult{
			ElicitationResponse: mcp.ElicitationResponse{Action: mcp.ElicitationResponseActionCancel},
		}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := server.NewMCPServer("test", "0.0.0", server.WithElicitation())
			session := &elicitSession{result: tt.result}
			ctx := s.WithContext(context.Background(), session)

			abort := confirmWrite(ctx, s, "Update issue #1", []string{summarize("Comment", "hello")})

			if tt.allowed && abort != nil {
				t.Fatalf("write should proceed, got abort")
			}
			if !tt.allowed && abort == nil {
				t.Fatalf("write should be aborted, got approval")
			}
			if session.calls != 1 {
				t.Fatalf("expected 1 elicitation request, got %d", session.calls)
			}
		})
	}
}

// A client that cannot elicit must never get a silent write.
func TestConfirmWriteWithoutElicitationSupport(t *testing.T) {
	s := server.NewMCPServer("test", "0.0.0", server.WithElicitation())

	for name, ctx := range map[string]context.Context{
		"no session":          context.Background(),
		"session without cap": s.WithContext(context.Background(), plainSession{}),
	} {
		t.Run(name, func(t *testing.T) {
			if abort := confirmWrite(ctx, s, "Update issue #1", nil); abort == nil {
				t.Fatal("write should be aborted when the user cannot be asked")
			}
		})
	}
}

// A client that did not advertise elicitation must be rejected without sending
// a request it may never answer.
func TestConfirmWriteUndeclaredCapability(t *testing.T) {
	s := server.NewMCPServer("test", "0.0.0", server.WithElicitation())
	session := &silentSession{elicitSession: elicitSession{result: accepted(map[string]any{"confirm": true})}}
	ctx := s.WithContext(context.Background(), session)

	if abort := confirmWrite(ctx, s, "Update issue #1", nil); abort == nil {
		t.Fatal("write should be aborted when the client never advertised elicitation")
	}
	if session.calls != 0 {
		t.Fatalf("no elicitation should be sent, got %d", session.calls)
	}
}

// silentSession can technically receive elicitation requests but never
// advertised the capability.
type silentSession struct{ elicitSession }

func (s *silentSession) GetClientCapabilities() mcp.ClientCapabilities {
	return mcp.ClientCapabilities{}
}

func TestConfirmWriteAutoWrite(t *testing.T) {
	t.Setenv("REDMINE_AUTO_WRITE", "1")

	s := server.NewMCPServer("test", "0.0.0", server.WithElicitation())
	session := &elicitSession{result: accepted(map[string]any{"confirm": false})}
	ctx := s.WithContext(context.Background(), session)

	if abort := confirmWrite(ctx, s, "Update issue #1", nil); abort != nil {
		t.Fatal("REDMINE_AUTO_WRITE should bypass confirmation")
	}
	if session.calls != 0 {
		t.Fatalf("no elicitation expected, got %d", session.calls)
	}
}

func TestSummarize(t *testing.T) {
	if got := summarize("Comment", "  "); got != "" {
		t.Fatalf("empty value should be skipped, got %q", got)
	}
	if got := summarize("Comment", " hi "); got != "Comment: hi" {
		t.Fatalf("got %q", got)
	}

	long := strings.Repeat("é", maxPreview+10)
	got := summarize("Description", long)
	if !strings.Contains(got, "truncated, 1510 characters total") {
		t.Fatalf("expected a truncation notice, got %q", got[len(got)-60:])
	}
	if strings.Contains(got, "�") {
		t.Fatal("truncation broke a multi-byte rune")
	}
}
