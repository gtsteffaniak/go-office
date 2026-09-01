package convert

import "context"

// X2TRunner executes an x2t task file. Production code uses the real x2t binary;
// tests may inject a fake runner to assert task XML without running x2t.
type X2TRunner func(ctx context.Context, taskPath string, isolatedDir string) ([]byte, error)
