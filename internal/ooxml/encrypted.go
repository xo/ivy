package ooxml

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/xo/magic/cfb"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/offcrypto"
)

// decrypt returns the package of an Office Open XML file that a password
// protects. Such a file is a Compound File with the streams EncryptionInfo
// and EncryptedPackage (MS-OFFCRYPTO). decrypt tries the password of cfg,
// and while the password is wrong, it asks PasswordFunc for another one, as
// D12 says. It returns an error that wraps ivy.ErrPassword if no password
// is right.
func decrypt(ctx context.Context, src ivy.Source, cfg ivy.Config) ([]byte, error) {
	lim := cfg.Limits.WithDefaults()
	f, err := cfb.Open(src, src.Size(), cfb.Limits{Entries: lim.MaxObjects, StreamSize: lim.MaxMemory})
	if err != nil {
		return nil, cfbError(err)
	}
	buf, err := f.ReadFile("EncryptionInfo")
	if err != nil {
		return nil, cfbError(err)
	}
	info, err := offcrypto.ParseEncryptionInfo(buf)
	if err != nil {
		return nil, err
	}
	pkg, err := f.Open("EncryptedPackage")
	if err != nil {
		return nil, cfbError(err)
	}
	key, err := info.Key(ctx, cfg.Password)
	for attempt := 1; err != nil && errors.Is(err, ivy.ErrPassword) && cfg.PasswordFunc != nil; attempt++ {
		password, perr := cfg.PasswordFunc(ctx, attempt)
		if perr != nil {
			return nil, fmt.Errorf("opening office open xml: %w: %w", ivy.ErrPassword, perr)
		}
		key, err = info.Key(ctx, password)
	}
	if err != nil {
		return nil, fmt.Errorf("opening office open xml: %w", err)
	}
	return key.Decrypt(ctx, pkg, pkg.Size(), lim)
}

// cfbError wraps an error of the reader of Compound Files in the error of
// ivy that matches it.
func cfbError(err error) error {
	switch {
	case errors.Is(err, cfb.ErrLimit):
		return fmt.Errorf("reading the compound file: %w: %w", ivy.ErrLimit, err)
	case errors.Is(err, cfb.ErrCorrupt):
		return fmt.Errorf("reading the compound file: %w: %w", ivy.ErrFormat, err)
	}
	return fmt.Errorf("reading the compound file: %w: %w", ivy.ErrFormat, err)
}

// isCompoundFile reports whether src starts with the signature of a
// Compound File.
func isCompoundFile(src ivy.Source) bool {
	head := make([]byte, len(cfbSignature))
	_, err := src.ReadAt(head, 0)
	return err == nil && bytes.Equal(head, cfbSignature)
}
