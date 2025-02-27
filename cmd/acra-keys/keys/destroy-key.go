/*
 * Copyright 2020, Cossack Labs Limited
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package keys

import (
	"errors"
	"flag"
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"

	"github.com/cossacklabs/acra/cmd"
	"github.com/cossacklabs/acra/keystore"
)

// SupportedDestroyKeyKinds is a list of keys supported by `destroy-key` subcommand.
var SupportedDestroyKeyKinds = []string{}

// ErrInvalidIndex error represent invalid index for --index flag
var ErrInvalidIndex = errors.New("invalid index value provided")

// DestroyKeyParams are parameters of "acra-keys destroy" subcommand.
type DestroyKeyParams interface {
	DestroyKeyKind() string
	ClientID() []byte
	Index() int
}

// DestroyKeySubcommand is the "acra-keys destroy" subcommand.
type DestroyKeySubcommand struct {
	CommonKeyStoreParameters
	FlagSet *flag.FlagSet

	index          int
	destroyKeyKind string
	contextID      []byte
}

// Name returns the same of this subcommand.
func (p *DestroyKeySubcommand) Name() string {
	return CmdDestroyKey
}

// GetFlagSet returns flag set of this subcommand.
func (p *DestroyKeySubcommand) GetFlagSet() *flag.FlagSet {
	return p.FlagSet
}

// RegisterFlags registers command-line flags of "acra-keys read".
func (p *DestroyKeySubcommand) RegisterFlags() {
	p.FlagSet = flag.NewFlagSet(CmdReadKey, flag.ContinueOnError)
	p.CommonKeyStoreParameters.Register(p.FlagSet)
	p.FlagSet.IntVar(&p.index, "index", 1, "Index of key to destroy (1 - represents current key, 2..n - rotated key)")
	p.FlagSet.Usage = func() {
		fmt.Fprintf(os.Stderr, "Command \"%s\": destroy key material\n", CmdDestroyKey)
		fmt.Fprintf(os.Stderr, "\n\t%s %s [options...] <key-ID>\n\n", os.Args[0], CmdDestroyKey)
		fmt.Fprintf(os.Stderr, "\nOptions:\n")
		cmd.PrintFlags(p.FlagSet)
	}
}

// Parse command-line parameters of the subcommand.
func (p *DestroyKeySubcommand) Parse(arguments []string) error {
	err := cmd.ParseFlagsWithConfig(p.FlagSet, arguments, DefaultConfigPath, ServiceName)
	if err != nil {
		return err
	}
	args := p.FlagSet.Args()
	if len(args) < 1 {
		log.Errorf("\"%s\" command requires key kind", CmdDestroyKey)
		return ErrMissingKeyKind
	}
	// It makes sense to allow multiple keys, but currently we don't allow it.
	if len(args) > 1 {
		log.Errorf("\"%s\" command does not support more than one key kind", CmdDestroyKey)
		return ErrMultipleKeyKinds
	}

	if p.index <= 0 {
		log.Errorf("\"%s\" expected --index flag value greater than 1", CmdDestroyKey)
		return ErrInvalidIndex
	}

	coarseKind, id, err := ParseKeyKind(args[0])
	if err != nil {
		return err
	}
	switch coarseKind {
	case keystore.KeyPoisonKeypair, keystore.KeyPoisonSymmetric:
		p.destroyKeyKind = coarseKind

	case keystore.KeySymmetric, keystore.KeyStorageKeypair, keystore.KeySearch:
		p.destroyKeyKind = coarseKind
		p.contextID = id
	default:
		return ErrUnknownKeyKind
	}

	return nil
}

// Execute this subcommand.
func (p *DestroyKeySubcommand) Execute() {
	keyStore, err := OpenKeyStoreForWriting(p)
	if err != nil {
		log.WithError(err).Fatal("Failed to open keystore")
	}
	DestroyKeyCommand(p, keyStore)
}

// DestroyKeyKind returns requested kind of the key to destroy.
func (p *DestroyKeySubcommand) DestroyKeyKind() string {
	return p.destroyKeyKind
}

// ClientID returns client ID of the requested key.
func (p *DestroyKeySubcommand) ClientID() []byte {
	return p.contextID
}

// Index returns index of key to be destroyed.
func (p *DestroyKeySubcommand) Index() int {
	return p.index
}

// DestroyKeyCommand implements the "destroy" command.
func DestroyKeyCommand(params DestroyKeyParams, keyStore keystore.KeyMaking) {
	err := DestroyKey(params, keyStore)
	if err != nil {
		log.WithError(err).Fatal("Failed to destroy key")
	}
}

// DestroyKey destroys data of the requsted key.
func DestroyKey(params DestroyKeyParams, keyStore keystore.KeyMaking) error {
	kind := params.DestroyKeyKind()

	switch kind {
	case keystore.KeyPoisonKeypair:
		if index := params.Index(); index > 1 {
			if err := keyStore.DestroyRotatedPoisonKeyPair(index); err != nil {
				log.WithError(err).Error("Cannot destroy poison record rotated key pair by index")
				return err
			}

			return nil
		}

		err := keyStore.DestroyPoisonKeyPair()
		if err != nil {
			log.WithError(err).Error("Cannot destroy poison record key pair")
			return err
		}
		return nil
	case keystore.KeyPoisonSymmetric:
		if index := params.Index(); index > 1 {
			if err := keyStore.DestroyRotatedPoisonSymmetricKey(index); err != nil {
				log.WithError(err).Error("Cannot destroy poison record rotated symmetric key by index")
				return err
			}

			return nil
		}

		err := keyStore.DestroyPoisonSymmetricKey()
		if err != nil {
			log.WithError(err).Error("Cannot destroy poison record symmetric key")
			return err
		}
		return nil

	case keystore.KeyStorageKeypair:
		if index := params.Index(); index > 1 {
			if err := keyStore.DestroyRotatedClientIDEncryptionKeyPair(params.ClientID(), index); err != nil {
				log.WithError(err).Error("Cannot destroy client storage rotated key pair by index")
				return err
			}

			return nil
		}

		err := keyStore.DestroyClientIDEncryptionKeyPair(params.ClientID())
		if err != nil {
			log.WithError(err).Error("Cannot destroy client storage key pair")
			return err
		}
		return nil

	case keystore.KeySymmetric:
		if index := params.Index(); index > 1 {
			if err := keyStore.DestroyRotatedClientIDSymmetricKey(params.ClientID(), index); err != nil {
				log.WithError(err).Error("Cannot destroy client symmetric rotated key by index")
				return err
			}

			return nil
		}

		err := keyStore.DestroyClientIDSymmetricKey(params.ClientID())
		if err != nil {
			log.WithError(err).Error("Cannot destroy client symmetric key")
			return err
		}
		return nil
	case keystore.KeySearch:
		if index := params.Index(); index > 1 {
			if err := keyStore.DestroyRotatedHmacSecretKey(params.ClientID(), index); err != nil {
				log.WithError(err).Error("Cannot destroy client hmac rotated key by index")
				return err
			}

			return nil
		}

		err := keyStore.DestroyHmacSecretKey(params.ClientID())
		if err != nil {
			log.WithError(err).Error("Cannot destroy client hmac key")
			return err
		}
		return nil
	default:
		log.WithField("expected", SupportedDestroyKeyKinds).Errorf("Unknown key kind: %s", kind)
		return ErrUnknownKeyKind
	}
}
