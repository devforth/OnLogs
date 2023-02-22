package agent

import "testing"

func TestSendInitRequest(t *testing.T) {
	defer func() {
		r := recover().(string)
		if r != "ERROR: Can't send request to host: Post \"/api/v1/addHost\": unsupported protocol scheme \"\"" {
			t.Error("Not expected error: ", r)
		}
	}()
	SendInitRequest([]string{})
}
