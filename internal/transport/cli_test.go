package transport

import "testing"

const unwrapped = `{"uid":"u1","name":{"ok":true,"value":"x.bundle"},"type":"file",
 "activeRevision":{"uid":"r1","state":"active","claimedSize":8}}`

const wrapped = `{"uid":"u1","name":{"ok":true,"value":"x.bundle"},"type":"file",
 "activeRevision":{"ok":true,"value":{"uid":"r1","state":"active","claimedSize":8}}}`

func TestParseNodeJSONBothShapes(t *testing.T) {
	for name, payload := range map[string]string{"0.7.0": unwrapped, "0.4.6": wrapped} {
		n, err := parseNodeJSON([]byte(payload))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if n.Name != "x.bundle" || n.Size != 8 {
			t.Errorf("%s: got %+v", name, n)
		}
	}
}
