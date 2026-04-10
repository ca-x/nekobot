package tools

import (
	"context"
	"reflect"
	"testing"
	"time"
)

type testHookTool struct{}

func (t testHookTool) Name() string { return "hook_tool" }
func (t testHookTool) Description() string { return "hook tool" }
func (t testHookTool) Parameters() map[string]interface{} { return map[string]interface{}{} }
func (t testHookTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	return "ok", nil
}

func TestRegistryRunsBeforeAndAfterHooks(t *testing.T) {
	reg := NewRegistry()
	reg.MustRegister(testHookTool{})

	var events []string
	reg.SetBeforeHook(func(ctx context.Context, toolName string, args map[string]interface{}) {
		events = append(events, "before:"+toolName)
	})
	reg.SetHook(func(ctx context.Context, toolName string, args map[string]interface{}, result string, _ time.Duration, err error) {
		events = append(events, "after:"+toolName+":"+result)
	})

	result, err := reg.Execute(context.Background(), "hook_tool", map[string]interface{}{"x": "y"})
	if err != nil {
		t.Fatalf("execute tool: %v", err)
	}
	if result != "ok" {
		t.Fatalf("expected result ok, got %q", result)
	}

	want := []string{"before:hook_tool", "after:hook_tool:ok"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("unexpected hook order: got %+v want %+v", events, want)
	}
}
