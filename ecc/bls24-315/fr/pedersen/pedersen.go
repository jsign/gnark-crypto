// Copyright 2020 ConsenSys Software Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Code generated by consensys/gnark-crypto DO NOT EDIT

package pedersen

import (
	"crypto/rand"
	"fmt"
	"github.com/consensys/gnark-crypto/ecc"
	curve "github.com/consensys/gnark-crypto/ecc/bls24-315"
	"github.com/consensys/gnark-crypto/ecc/bls24-315/fr"
	"io"
	"math/big"
)

// ProvingKey for committing and proofs of knowledge
type ProvingKey struct {
	basis         []curve.G1Affine
	basisExpSigma []curve.G1Affine
}

type VerifyingKey struct {
	g             curve.G2Affine // TODO @tabaie: does this really have to be randomized?
	gRootSigmaNeg curve.G2Affine //gRootSigmaNeg = g^{-1/σ}
}

func randomFrSizedBytes() ([]byte, error) {
	res := make([]byte, fr.Bytes)
	_, err := rand.Read(res)
	return res, err
}

func randomOnG2() (curve.G2Affine, error) { // TODO: Add to G2.go?
	if gBytes, err := randomFrSizedBytes(); err != nil {
		return curve.G2Affine{}, err
	} else {
		return curve.HashToG2(gBytes, []byte("random on g2"))
	}
}

func Setup(basis []curve.G1Affine) (pk ProvingKey, vk VerifyingKey, err error) {

	if vk.g, err = randomOnG2(); err != nil {
		return
	}

	var modMinusOne big.Int
	modMinusOne.Sub(fr.Modulus(), big.NewInt(1))
	var sigma *big.Int
	if sigma, err = rand.Int(rand.Reader, &modMinusOne); err != nil {
		return
	}
	sigma.Add(sigma, big.NewInt(1))

	var sigmaInvNeg big.Int
	sigmaInvNeg.ModInverse(sigma, fr.Modulus())
	sigmaInvNeg.Sub(fr.Modulus(), &sigmaInvNeg)
	vk.gRootSigmaNeg.ScalarMultiplication(&vk.g, &sigmaInvNeg)

	pk.basisExpSigma = make([]curve.G1Affine, len(basis))
	for i := range basis {
		pk.basisExpSigma[i].ScalarMultiplication(&basis[i], sigma)
	}

	pk.basis = basis
	return
}

func (pk *ProvingKey) Commit(values []fr.Element) (commitment curve.G1Affine, knowledgeProof curve.G1Affine, err error) {

	if len(values) != len(pk.basis) {
		err = fmt.Errorf("unexpected number of values")
		return
	}

	// TODO @gbotrel this will spawn more than one task, see
	// https://github.com/ConsenSys/gnark-crypto/issues/269
	config := ecc.MultiExpConfig{
		NbTasks: 1, // TODO Experiment
	}

	if _, err = commitment.MultiExp(pk.basis, values, config); err != nil {
		return
	}

	_, err = knowledgeProof.MultiExp(pk.basisExpSigma, values, config)

	return
}

// Verify checks if the proof of knowledge is valid
func (vk *VerifyingKey) Verify(commitment curve.G1Affine, knowledgeProof curve.G1Affine) error {

	if !commitment.IsInSubGroup() || !knowledgeProof.IsInSubGroup() {
		return fmt.Errorf("subgroup check failed")
	}

	product, err := curve.Pair([]curve.G1Affine{commitment, knowledgeProof}, []curve.G2Affine{vk.g, vk.gRootSigmaNeg})
	if err != nil {
		return err
	}
	if product.IsOne() {
		return nil
	}
	return fmt.Errorf("proof rejected")
}

// Marshal

func (pk *ProvingKey) WriteTo(w io.Writer) (int64, error) {
	enc := curve.NewEncoder(w)

	if err := enc.Encode(pk.basis); err != nil {
		return enc.BytesWritten(), err
	}

	err := enc.Encode(pk.basisExpSigma)

	return enc.BytesWritten(), err
}

func (pk *ProvingKey) ReadFrom(r io.Reader) (int64, error) {
	dec := curve.NewDecoder(r)

	if err := dec.Decode(&pk.basis); err != nil {
		return dec.BytesRead(), err
	}
	if err := dec.Decode(&pk.basisExpSigma); err != nil {
		return dec.BytesRead(), err
	}

	if cL, pL := len(pk.basis), len(pk.basisExpSigma); cL != pL {
		return dec.BytesRead(), fmt.Errorf("commitment basis size (%d) doesn't match proof basis size (%d)", cL, pL)
	}

	return dec.BytesRead(), nil
}

func (vk *VerifyingKey) WriteTo(w io.Writer) (int64, error) {
	enc := curve.NewEncoder(w)
	var err error

	if err = enc.Encode(&vk.g); err != nil {
		return enc.BytesWritten(), err
	}
	err = enc.Encode(&vk.gRootSigmaNeg)
	return enc.BytesWritten(), err
}

func (vk *VerifyingKey) ReadFrom(r io.Reader) (int64, error) {
	dec := curve.NewDecoder(r)
	var err error

	if err = dec.Decode(&vk.g); err != nil {
		return dec.BytesRead(), err
	}
	err = dec.Decode(&vk.gRootSigmaNeg)
	return dec.BytesRead(), err
}
