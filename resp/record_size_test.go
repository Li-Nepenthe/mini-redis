package resp

import (
	"context"
	"strings"
	"testing"
)

func TestParserReportsActualWireRecordSizes(t *testing.T) {
	commands := []string{"*01\r\n$003\r\nGET\r\n", "*0\r\n"}
	i := 0
	for payload := range NewRespParser().ParseStream(context.Background(), strings.NewReader(strings.Join(commands, ""))) {
		if payload.Err != nil || payload.BytesRead != len(commands[i]) {
			t.Fatalf("record %d: bytes=%d err=%v", i, payload.BytesRead, payload.Err)
		}
		i++
	}
	if i != len(commands) {
		t.Fatal("missing record")
	}
}
