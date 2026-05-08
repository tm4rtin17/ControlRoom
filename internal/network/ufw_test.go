package network

import "testing"

func TestParseUFWStatus(t *testing.T) {
	t.Parallel()
	input := []byte(`Status: active
Logging: on (low)
Default: deny (incoming), allow (outgoing), disabled (routed)
New profiles: skip

     To                         Action      From
     --                         ------      ----
[ 1] 22                         ALLOW IN    Anywhere
[ 2] 80,443/tcp                 ALLOW IN    Anywhere
[ 3] 22 (v6)                    ALLOW IN    Anywhere (v6)
`)
	st := parseUFWStatus(input)
	if !st.Active {
		t.Fatal("expected active")
	}
	if len(st.Rules) != 3 {
		t.Fatalf("expected 3 rules, got %d", len(st.Rules))
	}
	if st.Rules[0].To != "22" || st.Rules[0].Action != "ALLOW IN" || st.Rules[0].From != "Anywhere" {
		t.Errorf("rule 1 wrong: %+v", st.Rules[0])
	}
	if st.Rules[1].To != "80,443/tcp" {
		t.Errorf("rule 2 wrong: %+v", st.Rules[1])
	}
	if st.Rules[2].From != "Anywhere (v6)" {
		t.Errorf("rule 3 wrong: %+v", st.Rules[2])
	}
}

func TestAddRuleValidation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		spec AddRuleSpec
		bad  bool
	}{
		{"basic allow port", AddRuleSpec{Action: "allow", Port: "22", Protocol: "tcp"}, false},
		{"allow from cidr", AddRuleSpec{Action: "allow", Port: "22", Protocol: "tcp", From: "192.168.1.0/24"}, false},
		{"bad action", AddRuleSpec{Action: "drop", Port: "22"}, true},
		{"port injection", AddRuleSpec{Action: "allow", Port: "22; rm -rf /"}, true},
		{"bad protocol", AddRuleSpec{Action: "allow", Port: "22", Protocol: "icmp"}, true},
		{"comma ports", AddRuleSpec{Action: "allow", Port: "80,443", Protocol: "tcp"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.spec.validate()
			if tt.bad && err == nil {
				t.Errorf("expected error")
			}
			if !tt.bad && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
