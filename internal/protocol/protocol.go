package protocol

import (
	"fmt"
	"strings"
)

type RefUpdate struct {
	Src   string // empty means delete
	Dst   string
	Force bool
}

func ParsePushBatch(lines []string) ([]RefUpdate, error) {
	var out []RefUpdate
	for _, l := range lines {
		if !strings.HasPrefix(l, "push ") {
			return nil, fmt.Errorf("not a push line: %q", l)
		}
		spec := strings.TrimPrefix(l, "push ")
		u := RefUpdate{}
		if strings.HasPrefix(spec, "+") {
			u.Force = true
			spec = spec[1:]
		}
		parts := strings.SplitN(spec, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("malformed refspec: %q", spec)
		}
		u.Src, u.Dst = parts[0], parts[1]
		if u.Dst == "" {
			return nil, fmt.Errorf("empty destination in %q", spec)
		}
		out = append(out, u)
	}
	return out, nil
}

// poisonOptions are options git sends whose REJECTION IT IGNORES. Replying
// "unsupported" does not stop the operation, so the batch must be refused later.
// Only `atomic` is genuinely honoured at the option response.
var poisonOptions = []string{
	"cas", "depth", "deepen-since", "deepen-not", "deepen-relative",
	"update-shallow", "filter", "from-promisor", "no-dependents", "refetch",
}

type Options struct {
	Poisoned string // name of the first unsupported safety option seen
}

func (o *Options) Observe(line string) {
	if !strings.HasPrefix(line, "option ") || o.Poisoned != "" {
		return
	}
	name := strings.Fields(strings.TrimPrefix(line, "option "))
	if len(name) == 0 {
		return
	}
	for _, p := range poisonOptions {
		if name[0] == p {
			o.Poisoned = p
			return
		}
	}
}
