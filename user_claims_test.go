/*
 * Copyright 2018 The NATS Authors
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

package jwt

import (
	"fmt"
	"testing"
	"time"

	"github.com/nats-io/nkeys"
)

func TestNewUserClaims(t *testing.T) {
	akp := createAccountNKey(t)
	ukp := createUserNKey(t)

	uc := NewUserClaims(publicKey(ukp, t))
	uc.Expires = time.Now().Add(time.Duration(time.Hour)).Unix()
	uJwt := encode(uc, akp, t)

	uc2, err := DecodeUserClaims(uJwt)
	if err != nil {
		t.Fatal("failed to decode uc", err)
	}

	AssertEquals(uc.String(), uc2.String(), t)

	AssertEquals(uc.Claims() != nil, true, t)
	AssertEquals(uc.Payload() != nil, true, t)
}

func TestUserClaimIssuer(t *testing.T) {
	akp := createAccountNKey(t)
	ukp := createUserNKey(t)

	uc := NewUserClaims(publicKey(ukp, t))
	uc.Expires = time.Now().Add(time.Duration(time.Hour)).Unix()
	uJwt := encode(uc, akp, t)

	temp, err := DecodeGeneric(uJwt)
	if err != nil {
		t.Fatal("failed to decode", err)
	}

	type kpInputs struct {
		name string
		kp   nkeys.KeyPair
		ok   bool
	}

	inputs := []kpInputs{
		{"account", createAccountNKey(t), true},
		{"user", createUserNKey(t), false},
		{"operator", createOperatorNKey(t), false},
		{"server", createServerNKey(t), false},
		{"cluster", createClusterNKey(t), false},
	}

	for _, i := range inputs {
		bad := encode(temp, i.kp, t)
		_, err = DecodeUserClaims(bad)
		if i.ok && err != nil {
			t.Fatal(fmt.Sprintf("unexpected error for %q: %v", i.name, err))
		}
		if !i.ok && err == nil {
			t.Logf("should have failed to decode user signed by %q", i.name)
			t.Fail()
		}
	}
}

func TestUserSubjects(t *testing.T) {
	type kpInputs struct {
		name string
		kp   nkeys.KeyPair
		ok   bool
	}

	inputs := []kpInputs{
		{"account", createAccountNKey(t), false},
		{"cluster", createClusterNKey(t), false},
		{"operator", createOperatorNKey(t), false},
		{"server", createServerNKey(t), false},
		{"user", createUserNKey(t), true},
	}

	for _, i := range inputs {
		c := NewUserClaims(publicKey(i.kp, t))
		_, err := c.Encode(createAccountNKey(t))
		if i.ok && err != nil {
			t.Fatal(fmt.Sprintf("unexpected error for %q: %v", i.name, err))
		}
		if !i.ok && err == nil {
			t.Logf("should have failed to encode user with with %q subject", i.name)
			t.Fail()
		}
	}
}

func TestNewNilUserClaim(t *testing.T) {
	v := NewUserClaims("")
	if v != nil {
		t.Fatal(fmt.Sprintf("expected nil user claim"))
	}
}

func TestUserType(t *testing.T) {
	c := NewUserClaims(publicKey(createUserNKey(t), t))
	s := encode(c, createAccountNKey(t), t)
	u, err := DecodeUserClaims(s)
	if err != nil {
		t.Fatalf("failed to decode user claim: %v", err)
	}

	if UserClaim != u.Type {
		t.Fatalf("user type is unexpected %q", u.Type)
	}
}

func TestSubjects(t *testing.T) {
	s := StringList{}
	if len(s) != 0 {
		t.Fatalf("expected len 0")
	}
	if s.Contains("a") {
		t.Fatalf("didn't expect 'a'")
	}
	s.Add("a")
	if !s.Contains("a") {
		t.Fatalf("expected 'a'")
	}
	s.Remove("a")
	if s.Contains("a") {
		t.Fatalf("didn't expect 'a' after removing")
	}
}

func TestUserValidation(t *testing.T) {
	ukp := createUserNKey(t)

	uc := NewUserClaims(publicKey(ukp, t))
	uc.Permissions.Pub.Allow.Add("a")
	uc.Permissions.Pub.Deny.Add("b")
	uc.Permissions.Sub.Allow.Add("a")
	uc.Permissions.Sub.Deny.Add("b")
	uc.Limits.Max = 10
	uc.Limits.Payload = 10
	uc.Limits.Src = "1.1.1.1"
	uc.Limits.Times = []TimeRange{
		{
			Start: "01:15:00",
			End:   "03:15:00",
		},
		{
			Start: "06:15:00",
			End:   "09:15:00",
		},
	}

	vr := CreateValidationResults()
	uc.Validate(vr)

	if !vr.IsEmpty() {
		t.Error("valid user permissions should be valid")
	}
	uc.Limits.Max = -1
	vr = CreateValidationResults()
	uc.Validate(vr)

	if vr.IsEmpty() || len(vr.Issues) != 1 || !vr.IsBlocking(true) {
		t.Error("bad limit should be invalid")
	}

	uc.Limits.Max = 10
	uc.Limits.Payload = -1
	vr = CreateValidationResults()
	uc.Validate(vr)

	if vr.IsEmpty() || len(vr.Issues) != 1 || !vr.IsBlocking(true) {
		t.Error("bad limit should be invalid")
	}

	uc.Limits.Payload = 10
	uc.Limits.Src = "hello world"
	vr = CreateValidationResults()
	uc.Validate(vr)

	if vr.IsEmpty() || len(vr.Issues) != 1 || !vr.IsBlocking(true) {
		t.Error("bad limit should be invalid")
	}

	uc.Limits.Payload = 10
	uc.Limits.Src = "hello world"
	vr = CreateValidationResults()
	uc.Validate(vr)

	if vr.IsEmpty() || len(vr.Issues) != 1 || !vr.IsBlocking(true) {
		t.Error("bad limit should be invalid")
	}

	tr := TimeRange{
		Start: "hello",
		End:   "03:15:00",
	}
	uc.Limits.Src = "1.1.1.1"
	uc.Limits.Times = append(uc.Limits.Times, tr)
	vr = CreateValidationResults()
	uc.Validate(vr)

	if vr.IsEmpty() || len(vr.Issues) != 1 || !vr.IsBlocking(true) {
		t.Error("bad limit should be invalid")
	}

	uc.Limits.Times = []TimeRange{}
	uc.Permissions.Pub.Allow.Add("bad subject")
	vr = CreateValidationResults()
	uc.Validate(vr)

	if vr.IsEmpty() || len(vr.Issues) != 1 || !vr.IsBlocking(true) {
		t.Error("bad permission should be invalid")
	}

	uc.Permissions.Pub.Allow.Remove("bad subject")
	uc.Permissions.Sub.Allow.Add("bad subject")
	vr = CreateValidationResults()
	uc.Validate(vr)

	if vr.IsEmpty() || len(vr.Issues) != 1 || !vr.IsBlocking(true) {
		t.Error("bad permission should be invalid")
	}

	uc.Permissions.Sub.Allow.Remove("bad subject")
	uc.Permissions.Pub.Deny.Add("bad subject")
	vr = CreateValidationResults()
	uc.Validate(vr)

	if vr.IsEmpty() || len(vr.Issues) != 1 || !vr.IsBlocking(true) {
		t.Error("bad permission should be invalid")
	}

	uc.Permissions.Pub.Deny.Remove("bad subject")
	uc.Permissions.Sub.Deny.Add("bad subject")
	vr = CreateValidationResults()
	uc.Validate(vr)

	if vr.IsEmpty() || len(vr.Issues) != 1 || !vr.IsBlocking(true) {
		t.Error("bad permission should be invalid")
	}
}
