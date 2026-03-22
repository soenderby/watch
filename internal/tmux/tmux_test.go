package tmux

import (
	"testing"
	"time"
)

func TestParseSessionLine(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		want    Session
		wantErr bool
	}{
		{
			name: "full line",
			line: "myproject|3|1710936000|1|1710936060|/home/user/code/myproject",
			want: Session{
				Name:     "myproject",
				Windows:  3,
				Created:  time.Unix(1710936000, 0),
				Attached: true,
				Activity: time.Unix(1710936060, 0),
				Path:     "/home/user/code/myproject",
			},
		},
		{
			name: "not attached",
			line: "orca-agent-1|1|1710936000|0|1710936030|/home/user/code/orca/worktrees/agent-1",
			want: Session{
				Name:     "orca-agent-1",
				Windows:  1,
				Created:  time.Unix(1710936000, 0),
				Attached: false,
				Activity: time.Unix(1710936030, 0),
				Path:     "/home/user/code/orca/worktrees/agent-1",
			},
		},
		{
			name: "empty path",
			line: "scratch|1|1710936000|0|1710936000|",
			want: Session{
				Name:     "scratch",
				Windows:  1,
				Created:  time.Unix(1710936000, 0),
				Attached: false,
				Activity: time.Unix(1710936000, 0),
				Path:     "",
			},
		},
		{
			name:    "too few fields",
			line:    "name|1|123",
			wantErr: true,
		},
		{
			name: "non-numeric windows",
			line: "name|abc|1710936000|0|1710936000|/tmp",
			want: Session{
				Name:     "name",
				Windows:  0, // defaults to 0 on parse error
				Created:  time.Unix(1710936000, 0),
				Attached: false,
				Activity: time.Unix(1710936000, 0),
				Path:     "/tmp",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSessionLine(tt.line)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Name != tt.want.Name {
				t.Errorf("Name: got %q, want %q", got.Name, tt.want.Name)
			}
			if got.Windows != tt.want.Windows {
				t.Errorf("Windows: got %d, want %d", got.Windows, tt.want.Windows)
			}
			if !got.Created.Equal(tt.want.Created) {
				t.Errorf("Created: got %v, want %v", got.Created, tt.want.Created)
			}
			if got.Attached != tt.want.Attached {
				t.Errorf("Attached: got %v, want %v", got.Attached, tt.want.Attached)
			}
			if !got.Activity.Equal(tt.want.Activity) {
				t.Errorf("Activity: got %v, want %v", got.Activity, tt.want.Activity)
			}
			if got.Path != tt.want.Path {
				t.Errorf("Path: got %q, want %q", got.Path, tt.want.Path)
			}
		})
	}
}
