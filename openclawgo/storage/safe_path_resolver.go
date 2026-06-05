package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

var (
	safePathSegmentRegex = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
)

type ISafePathResolver interface {
	ResolveSafePath(scopeRoot, requestedPath string) (string, error)
}

var ReservedNames = map[string]struct{}{
	"CON": {}, "PRN": {}, "AUX": {}, "NUL": {},
	"COM1": {}, "COM2": {}, "COM3": {}, "COM4": {}, "COM5": {}, "COM6": {}, "COM7": {}, "COM8": {}, "COM9": {},
	"LPT1": {}, "LPT2": {}, "LPT3": {}, "LPT4": {}, "LPT5": {}, "LPT6": {}, "LPT7": {}, "LPT8": {}, "LPT9": {},
}

var _ ISafePathResolver = (*SafePathResolver)(nil)

type SafePathResolver struct {
}

// ResolveSafePath implements [ISafePathResolver].
func (s *SafePathResolver) ResolveSafePath(scopeRoot string, requestedPath string) (string, error) {
	// ---- input validation ------------------------------------------------
	if len(scopeRoot) == 0 {
		return "", errors.New("scope root must be a non-empty rooted path")
	}

	if len(requestedPath) == 0 {
		return "", errors.New("requested path must be non-empty")
	}

	for _, ch := range requestedPath {
		if unicode.IsControl(ch) {
			return "", fmt.Errorf("requested path contains an illegal control character")
		}
	}

	if !filepath.IsAbs(requestedPath) {
		if err := s.validateRawSegments(requestedPath, scopeRoot); err != nil {
			return "", err
		}
	}
	normalizedScope, err := filepath.Abs(scopeRoot)
	if err != nil {
		return "", fmt.Errorf("scope root is not a valid path: %s", err.Error())
	}
	if !filepath.IsAbs(normalizedScope) {
		if err := s.validateRawSegments(requestedPath, scopeRoot); err != nil {
			return "", fmt.Errorf("scope root must be an absolute (rooted) path")
		}
	}

	normalizedCombined, err := filepath.Abs(filepath.Join(normalizedScope, requestedPath))
	if err != nil {
		return "", fmt.Errorf("requested path is not valid: %w", err)
	}
	if !s.isWithinScope(normalizedScope, normalizedCombined) {
		return "", errors.New("requested path resolves outside the scope root")
	}

	// ---- H-5 segment-name validation -------------------------------------
	if err := s.validateSegmentsBelowScope(normalizedScope, normalizedCombined, requestedPath); err != nil {
		return "", err
	}

	// ---- H-3 reparse-point escape check ----------------------------------
	if err := s.ensureNoReparsePointEscape(normalizedScope, normalizedCombined, requestedPath); err != nil {
		return "", err
	}

	return normalizedCombined, nil
}

func (s *SafePathResolver) isWithinScope(scope, target string) bool {
	scopeAbs, err := filepath.Abs(scope)
	if err != nil {
		return false
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return false
	}

	if scopeAbs == targetAbs {
		return true
	}

	if !strings.HasSuffix(scopeAbs, string(filepath.Separator)) {
		scopeAbs += string(filepath.Separator)
	}

	return strings.HasPrefix(targetAbs, scopeAbs)
}

func (s *SafePathResolver) validateSegmentName(name, scopeRoot, requestedPath string) error {
	if len(name) == 0 {
		return errors.New("path segment is empty")
	}

	if name[0] == '.' || name[0] == ' ' || name[len(name)-1] == '.' || name[len(name)-1] == ' ' {
		return fmt.Errorf("path segment '%s' violates the safe-name policy", name)
	}

	if !safePathSegmentRegex.MatchString(name) {
		return fmt.Errorf("path segment '%s' violates the safe-name policy", name)
	}
	before, _, ok := strings.Cut(name, ".")
	stem := name
	if ok {
		stem = before
	}

	if _, ok := ReservedNames[stem]; ok {
		return fmt.Errorf("path segment '%s' uses a reserved Windows device name", name)
	}
	return nil
}

func (s *SafePathResolver) validateRawSegments(requestedPath, scopeRoot string) error {
	segments := strings.FieldsFunc(requestedPath, func(r rune) bool {
		return r == '\\' || r == '/'
	})

	for _, seg := range segments {

		if seg == "." || seg == ".." {
			continue
		}

		if err := s.validateSegmentName(seg, scopeRoot, requestedPath); err != nil {
			return err
		}
	}

	return nil
}

func (s *SafePathResolver) validateSegmentsBelowScope(normalizedScope, normalizedCombined, requestedPath string) error {
	if normalizedScope == normalizedCombined {
		return nil
	}

	var tail = normalizedCombined[(len(normalizedScope) + 1):]
	segments := strings.FieldsFunc(tail, func(r rune) bool {
		return r == '\\' || r == '/'
	})

	for _, seg := range segments {

		if seg == "." || seg == ".." {
			continue
		}

		if err := s.validateSegmentName(seg, normalizedScope, requestedPath); err != nil {
			return err
		}
	}

	return nil
}

func (s *SafePathResolver) isValidSegmentName(name string) bool {
	if len(name) == 0 {
		return false
	}

	if name[0] == '.' || name[0] == ' ' {
		return false
	}
	if name[len(name)-1] == '.' || name[len(name)-1] == ' ' {
		return false
	}
	if !safePathSegmentRegex.MatchString(name) {
		return false
	}

	before, _, ok := strings.Cut(name, ".")
	stem := name
	if ok {
		stem = before
	}

	_, ok = ReservedNames[stem]
	return ok
}

func (s *SafePathResolver) ensureNoReparsePointEscape(normalizedScope, normalizedCombined, requestedPath string) error {
	if normalizedScope == normalizedCombined {
		return nil
	}

	tail := normalizedCombined[len(normalizedScope)+1:]
	segments := strings.FieldsFunc(tail, func(r rune) bool {
		return r == filepath.Separator || r == '/'
	})

	current := normalizedScope
	for _, seg := range segments {
		current = filepath.Join(current, seg)

		info, err := os.Lstat(current)
		if err != nil {
			continue
		}

		if info.Mode()&os.ModeSymlink == 0 {
			continue
		}

		finalTarget, err := filepath.EvalSymlinks(current)
		if err != nil {
			return errors.New("reparse point at '" + current + "' could not be resolved: " + err.Error())
		}

		finalNormalized, err := filepath.Abs(finalTarget)
		if err != nil {
			return errors.New("reparse-point target at '" + current + "' is not a valid path: " + err.Error())
		}

		finalNormalized = filepath.Clean(finalNormalized)

		if !s.isWithinScope(normalizedScope, finalNormalized) {
			return errors.New("reparse point at '" + current + "' escapes the scope root")
		}
	}

	return nil
}
