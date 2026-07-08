package task

import "testing"

func TestValidate(t *testing.T) {
	cases := []struct {
		name    string
		task    Task
		wantErr bool
	}{
		{"valid todo", Task{Title: "write tests", Status: StatusTodo}, false},
		{"valid doing", Task{Title: "x", Status: StatusDoing}, false},
		{"valid done", Task{Title: "x", Status: StatusDone}, false},
		{"missing title", Task{Status: StatusTodo}, true},
		{"bad status", Task{Title: "x", Status: "blocked"}, true},
		{"empty status", Task{Title: "x"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := c.task.Validate(); (err != nil) != c.wantErr {
				t.Fatalf("Validate() = %v, wantErr %v", err, c.wantErr)
			}
		})
	}
}
