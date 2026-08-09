package biceprpcclient

import "testing"

func TestArtifactName(t *testing.T) {
	tests := map[string]string{
		"windows/amd64": "bicep-win-x64.exe",
		"windows/arm64": "bicep-win-arm64.exe",
		"linux/amd64":   "bicep-linux-x64",
		"linux/arm64":   "bicep-linux-arm64",
		"darwin/amd64":  "bicep-osx-x64",
		"darwin/arm64":  "bicep-osx-arm64",
	}
	for platform, expected := range tests {
		t.Run(platform, func(t *testing.T) {
			separator := 0
			for index, character := range platform {
				if character == '/' {
					separator = index
					break
				}
			}
			actual, err := artifactName(platform[:separator], platform[separator+1:])
			if err != nil {
				t.Fatalf("artifactName returned an error: %v", err)
			}
			if actual != expected {
				t.Fatalf("artifactName returned %q, want %q", actual, expected)
			}
		})
	}
}

func TestArtifactNameRejectsUnsupportedPlatform(t *testing.T) {
	if _, err := artifactName("plan9", "amd64"); err == nil {
		t.Fatal("artifactName did not reject an unsupported platform")
	}
}
