package tests

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"terraform-provider-kion/internal/kgen/kfs/mocks"
)

// swapFS installs a fresh MockFS as the package's filesystem seam for the
// duration of the test, restoring the original on cleanup.
func swapFS(t *testing.T) *mocks.MockFS {
	t.Helper()
	m := mocks.NewMockFS(t)
	orig := fsw
	fsw = m
	t.Cleanup(func() { fsw = orig })
	return m
}

// statGoModThenMissing wires the mock's Stat so that:
//   - any path ending in "go.mod" is reported as existing (so findProjectRoot
//     resolves to the current working directory immediately), and
//   - every other path is reported as not-existing (so writeFileIfNeeded takes
//     the write branch rather than the skip branch).
func statGoModThenMissing(m *mocks.MockFS) {
	m.EXPECT().Stat(mock.Anything).RunAndReturn(func(name string) (fs.FileInfo, error) {
		if strings.HasSuffix(name, "go.mod") {
			return nil, nil
		}
		return nil, os.ErrNotExist
	}).Maybe()
}

// captureWrites installs MkdirAll/WriteFile expectations that record every
// written path (thread-safe) and return success. The returned func yields the
// captured paths.
func captureWrites(m *mocks.MockFS) func() []string {
	var mu sync.Mutex
	var written []string

	m.EXPECT().MkdirAll(mock.Anything, mock.Anything).Return(nil).Maybe()
	m.EXPECT().WriteFile(mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(name string, _ []byte, _ fs.FileMode) error {
			mu.Lock()
			written = append(written, name)
			mu.Unlock()
			return nil
		}).Maybe()

	return func() []string {
		mu.Lock()
		defer mu.Unlock()
		out := make([]string, len(written))
		copy(out, written)
		return out
	}
}

func hasSuffixInAny(paths []string, suffix string) bool {
	for _, p := range paths {
		if strings.HasSuffix(p, suffix) {
			return true
		}
	}
	return false
}

// TestGenerate_FilterResource_WritesExpectedFiles runs Generate filtered to a
// single resource and asserts the generated file set: the resource test file,
// the sweep file, and the sweep entrypoint.
func TestGenerate_FilterResource_WritesExpectedFiles(t *testing.T) {
	t.Chdir(t.TempDir())
	m := swapFS(t)
	statGoModThenMissing(m)
	writes := captureWrites(m)

	err := Generate(true /* force */, "kion_label", false /* sweepOnly */)
	require.NoError(t, err)

	got := writes()
	require.NotEmpty(t, got, "expected at least one file written")

	// The label service package test + sweep files.
	require.True(t, hasSuffixInAny(got, filepath.Join("internal", "service", "label", "label_test.go")),
		"expected label_test.go to be written, got %v", got)
	require.True(t, hasSuffixInAny(got, filepath.Join("internal", "service", "label", "sweep.go")),
		"expected label sweep.go to be written, got %v", got)

	// The sweep entrypoint is always written.
	require.True(t, hasSuffixInAny(got, filepath.Join("internal", "sweep", "sweep_test.go")),
		"expected sweep entrypoint sweep_test.go to be written, got %v", got)

	// The filter must exclude other resources' test files.
	require.False(t, hasSuffixInAny(got, filepath.Join("service", "cloud_rule", "cloud_rule_test.go")),
		"filter leaked cloud_rule test file, got %v", got)
}

// TestGenerate_SweepOnly writes only sweep files (no *_test.go resource test
// files, no data source test files) plus the sweep entrypoint.
func TestGenerate_SweepOnly(t *testing.T) {
	t.Chdir(t.TempDir())
	m := swapFS(t)
	statGoModThenMissing(m)
	writes := captureWrites(m)

	err := Generate(true, "kion_label", true /* sweepOnly */)
	require.NoError(t, err)

	got := writes()
	require.True(t, hasSuffixInAny(got, filepath.Join("service", "label", "sweep.go")),
		"expected sweep.go, got %v", got)
	require.True(t, hasSuffixInAny(got, filepath.Join("internal", "sweep", "sweep_test.go")),
		"expected sweep entrypoint, got %v", got)

	// sweepOnly must not emit the resource acceptance test file.
	require.False(t, hasSuffixInAny(got, filepath.Join("service", "label", "label_test.go")),
		"sweepOnly should not write label_test.go, got %v", got)
}

// TestGenerate_SkipWhenExists exercises the skip-if-exists branch: with
// force=false and Stat reporting every output path as existing, no MkdirAll or
// WriteFile calls should occur.
func TestGenerate_SkipWhenExists(t *testing.T) {
	t.Chdir(t.TempDir())
	m := swapFS(t)

	// Every path exists -> findProjectRoot resolves immediately AND every
	// output file is skipped. No MkdirAll/WriteFile expectations are set, so
	// the mock's AssertExpectations will fail if any write is attempted.
	m.EXPECT().Stat(mock.Anything).Return(nil, nil).Maybe()

	err := Generate(false /* force */, "kion_label", false)
	require.NoError(t, err)
}

// TestGenerate_WriteFile_Error propagates a WriteFile failure out of Generate.
func TestGenerate_WriteFile_Error(t *testing.T) {
	t.Chdir(t.TempDir())
	m := swapFS(t)
	statGoModThenMissing(m)
	m.EXPECT().MkdirAll(mock.Anything, mock.Anything).Return(nil).Maybe()

	sentinel := errors.New("disk full")
	m.EXPECT().WriteFile(mock.Anything, mock.Anything, mock.Anything).Return(sentinel).Maybe()

	err := Generate(true, "kion_label", false)
	require.Error(t, err)
	require.ErrorIs(t, err, sentinel)
}

// TestGenerate_MkdirAll_Error propagates a MkdirAll failure out of Generate.
func TestGenerate_MkdirAll_Error(t *testing.T) {
	t.Chdir(t.TempDir())
	m := swapFS(t)
	statGoModThenMissing(m)

	sentinel := errors.New("permission denied")
	m.EXPECT().MkdirAll(mock.Anything, mock.Anything).Return(sentinel).Maybe()
	// WriteFile should never be reached once MkdirAll fails; allow it anyway
	// to keep the mock lenient in case ordering changes.
	m.EXPECT().WriteFile(mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	err := Generate(true, "kion_label", false)
	require.Error(t, err)
	require.ErrorIs(t, err, sentinel)
}

// TestGenerate_NoMatchingResource confirms that a filter matching no
// resource or data source still succeeds (the sweep entrypoint is always
// generated, so the "generated == 0" guard never trips) and only writes the
// entrypoint file.
func TestGenerate_NoMatchingResource(t *testing.T) {
	t.Chdir(t.TempDir())
	m := swapFS(t)
	statGoModThenMissing(m)
	writes := captureWrites(m)

	err := Generate(true, "kion_this_resource_does_not_exist", false)
	require.NoError(t, err)

	got := writes()
	// The sweep entrypoint is unconditionally generated.
	require.True(t, hasSuffixInAny(got, filepath.Join("internal", "sweep", "sweep_test.go")),
		"expected sweep entrypoint even with a non-matching filter, got %v", got)
	// No resource/data-source/sweep files for any service should be written.
	require.False(t, hasSuffixInAny(got, filepath.Join("service", "label", "label_test.go")),
		"non-matching filter should not write resource files, got %v", got)
}

// TestGenerate_ProjectRootNotFound surfaces the findProjectRoot error when no
// go.mod is ever found walking up the tree.
func TestGenerate_ProjectRootNotFound(t *testing.T) {
	t.Chdir(t.TempDir())
	m := swapFS(t)
	// Nothing exists, ever -> findProjectRoot walks to the filesystem root and
	// fails.
	m.EXPECT().Stat(mock.Anything).Return(nil, os.ErrNotExist).Maybe()

	err := Generate(true, "kion_label", false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "could not find project root")
}

// TestGenerate_FullRun_NoFilter runs Generate across every registered package
// and asserts a broad set of files was produced, including a data source test
// file and the sweep entrypoint. This exercises the resource, data source, and
// sweep generators together.
func TestGenerate_FullRun_NoFilter(t *testing.T) {
	t.Chdir(t.TempDir())
	m := swapFS(t)
	statGoModThenMissing(m)
	writes := captureWrites(m)

	err := Generate(true, "" /* no filter */, false)
	require.NoError(t, err)

	got := writes()
	require.NotEmpty(t, got)

	// A resource test file for a well-known package.
	require.True(t, hasSuffixInAny(got, filepath.Join("service", "label", "label_test.go")),
		"expected label_test.go in full run")
	// The sweep entrypoint.
	require.True(t, hasSuffixInAny(got, filepath.Join("internal", "sweep", "sweep_test.go")),
		"expected sweep entrypoint in full run")
	// At least one data source test file should be emitted somewhere.
	require.True(t, hasSuffixInAny(got, "_data_source_test.go"),
		"expected at least one data source test file in full run")
}

// TestFindProjectRoot_WalksUp confirms findProjectRoot climbs parent
// directories until it finds a go.mod.
func TestFindProjectRoot_WalksUp(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "a", "b", "c")
	require.NoError(t, os.MkdirAll(child, 0750))
	t.Chdir(child)

	m := swapFS(t)
	goMod := filepath.Join(root, "go.mod")
	m.EXPECT().Stat(mock.Anything).RunAndReturn(func(name string) (fs.FileInfo, error) {
		if name == goMod {
			return nil, nil
		}
		return nil, os.ErrNotExist
	}).Maybe()

	got, err := findProjectRoot()
	require.NoError(t, err)
	require.Equal(t, root, got)
}

// TestWriteFileIfNeeded_SkipVsWrite directly exercises both branches of the
// skip-if-exists helper.
func TestWriteFileIfNeeded_SkipVsWrite(t *testing.T) {
	t.Run("skip when exists and not forced", func(t *testing.T) {
		m := swapFS(t)
		m.EXPECT().Stat("out.go").Return(nil, nil).Once()

		wrote, err := writeFileIfNeeded("out.go", "content", false)
		require.NoError(t, err)
		require.False(t, wrote)
	})

	t.Run("write when missing", func(t *testing.T) {
		m := swapFS(t)
		m.EXPECT().Stat("out.go").Return(nil, os.ErrNotExist).Once()
		m.EXPECT().MkdirAll(mock.Anything, mock.Anything).Return(nil).Once()
		m.EXPECT().WriteFile("out.go", []byte("content"), mock.Anything).Return(nil).Once()

		wrote, err := writeFileIfNeeded("out.go", "content", false)
		require.NoError(t, err)
		require.True(t, wrote)
	})

	t.Run("force overwrites without stat", func(t *testing.T) {
		m := swapFS(t)
		// With force=true, Stat must NOT be consulted for skip-if-exists.
		m.EXPECT().MkdirAll(mock.Anything, mock.Anything).Return(nil).Once()
		m.EXPECT().WriteFile("out.go", []byte("content"), mock.Anything).Return(nil).Once()

		wrote, err := writeFileIfNeeded("out.go", "content", true)
		require.NoError(t, err)
		require.True(t, wrote)
	})

	t.Run("mkdir error", func(t *testing.T) {
		m := swapFS(t)
		sentinel := errors.New("boom")
		m.EXPECT().MkdirAll(mock.Anything, mock.Anything).Return(sentinel).Once()

		wrote, err := writeFileIfNeeded("out.go", "content", true)
		require.Error(t, err)
		require.ErrorIs(t, err, sentinel)
		require.False(t, wrote)
	})

	t.Run("write error", func(t *testing.T) {
		m := swapFS(t)
		sentinel := errors.New("boom")
		m.EXPECT().MkdirAll(mock.Anything, mock.Anything).Return(nil).Once()
		m.EXPECT().WriteFile(mock.Anything, mock.Anything, mock.Anything).Return(sentinel).Once()

		wrote, err := writeFileIfNeeded("out.go", "content", true)
		require.Error(t, err)
		require.ErrorIs(t, err, sentinel)
		require.False(t, wrote)
	})
}
