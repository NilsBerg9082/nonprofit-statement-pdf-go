package statement

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildStatementSelectsDueRemindersAndTotalsMoney(t *testing.T) {
	tests := []struct {
		name      string
		input     Request
		want      []string
		doNotWant string
	}{
		{
			name: "includes reminders due on or before statement date",
			input: Request{
				AccountName: "River Pantry",
				Period:      "August 2026",
				AsOf:        "2026-08-31",
				Receipts: []Receipt{
					{Donor: "A. Chen", Amount: 2550, Issued: "2026-08-04"},
					{Donor: "M. Singh", Amount: 7449, Issued: "2026-08-20"},
				},
				Shifts: []VolunteerShift{
					{Volunteer: "Noah", Due: "2026-08-31", Task: "confirm Saturday shift"},
					{Volunteer: "Inez", Due: "2026-09-02", Task: "confirm delivery route"},
				},
				Campaigns: []Campaign{{Name: "School supplies", Raised: 183425}},
			},
			want:      []string{"2 receipts issued · $99.99 total", "Noah: confirm Saturday shift", "1 campaigns · $1834.25 raised", `{"requests":12}`},
			doNotWant: "Inez",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildStatement(tt.input, json.RawMessage(`{"requests":12}`))
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range tt.want {
				if !strings.Contains(got, want) {
					t.Errorf("statement missing %q\n%s", want, got)
				}
			}
			if strings.Contains(got, tt.doNotWant) {
				t.Errorf("future reminder %q was included", tt.doNotWant)
			}
		})
	}
}
