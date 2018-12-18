/*
 *    Copyright 2018 Insolar
 *
 *    Licensed under the Apache License, Version 2.0 (the "License");
 *    you may not use this file except in compliance with the License.
 *    You may obtain a copy of the License at
 *
 *        http://www.apache.org/licenses/LICENSE-2.0
 *
 *    Unless required by applicable law or agreed to in writing, software
 *    distributed under the License is distributed on an "AS IS" BASIS,
 *    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *    See the License for the specific language governing permissions and
 *    limitations under the License.
 */

package sign

import (
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/asn1"
	"math/big"

	"github.com/insolar/insolar/core"
	"github.com/pkg/errors"
)

const BigIntLength = 32

type ecdsaSignature struct {
	R, S *big.Int
}

func (p ecdsaSignature) Marshal() ([]byte, error) {
	signature, err := asn1.Marshal(p)
	if err != nil {
		return nil, errors.Wrap(err, "[ Marshall ] Could't marshal ecdsaSignature")
	}
	return signature, nil
}

func (p *ecdsaSignature) Unmarshal(signatureRaw []byte) error {
	rest, err := asn1.Unmarshal(signatureRaw, p)
	if len(rest) != 0 {
		return errors.New("[ Unmarshal ] len of rest must be 0")
	}
	if err != nil {
		return errors.Wrap(err, "[ Unmarshal ] Could't unmarshal ecdsaSignature")
	}
	return nil
}

type ecdsaSignerWrapper struct {
	privateKey *ecdsa.PrivateKey
	hasher     core.Hasher
}

func (sw *ecdsaSignerWrapper) Sign(data []byte) (*core.Signature, error) {
	hash := sw.hasher.Hash(data)

	r, s, err := ecdsa.Sign(rand.Reader, sw.privateKey, hash)
	if err != nil {
		return nil, errors.Wrap(err, "[ Sign ] could't sign data")
	}

	ecdsaSignature := makeSignature(r, s)
	if err != nil {
		return nil, errors.Wrap(err, "[ Sign ] could't sign data")
	}

	signature := core.SignatureFromBytes(ecdsaSignature)
	return &signature, nil
}

type ecdsaVerifyWrapper struct {
	publicKey *ecdsa.PublicKey
	hasher    core.Hasher
}

func (sw *ecdsaVerifyWrapper) Verify(signature core.Signature, data []byte) bool {
	r, s := getRSFromBytes(signature.Bytes())
	ecdsaSignature := ecdsaSignature{r, s}
	hash := sw.hasher.Hash(data)

	return ecdsa.Verify(sw.publicKey, hash, ecdsaSignature.R, ecdsaSignature.S)
}

func makeSignature(r, s *big.Int) []byte {
	if (len(r.Bytes()) > BigIntLength) ||
		(len(s.Bytes()) > BigIntLength) {
		panic("[ makeSignature ] wrong r, s length")
	}
	rLen := uint8(len(r.Bytes()))
	sLen := uint8(len(s.Bytes()))
	res := make([]byte, rLen+sLen+2) // 2 bytes for len
	res[0] = rLen
	copy(res[1:rLen+2], r.Bytes())
	res[rLen+1] = sLen
	copy(res[rLen+2:], s.Bytes())
	return res[:]
}

func getRSFromBytes(data []byte) (*big.Int, *big.Int) {
	if len(data) > (BigIntLength*2 + 2) {
		panic("[ getRSFromBytes ] wrong data length to get a r, s")
	}
	r := new(big.Int)
	s := new(big.Int)
	rLen := data[0]
	sLen := data[rLen+1]
	rBytes := make([]byte, rLen)
	sBytes := make([]byte, sLen)
	copy(rBytes, data[1:rLen+2])
	copy(sBytes, data[rLen+2:])
	r.SetBytes(rBytes)
	s.SetBytes(sBytes)
	return r, s
}
