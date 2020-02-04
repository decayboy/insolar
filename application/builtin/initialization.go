//
// Copyright 2019 Insolar Technologies GmbH
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
//

// Code generated by insgocc. DO NOT EDIT.
// source template in logicrunner/preprocessor/templates

package builtin

import (
	"github.com/pkg/errors"

	account "customappp/account"
	costcenter "customappp/costcenter"
	deposit "customappp/deposit"
	member "customappp/member"
	migrationadmin "customappp/migrationadmin"
	migrationdaemon "customappp/migrationdaemon"
	migrationshard "customappp/migrationshard"
	pkshard "customappp/pkshard"
	rootdomain "customappp/rootdomain"
	wallet "customappp/wallet"

	XXX_insolar "github.com/insolar/insolar/insolar"
	XXX_artifacts "github.com/insolar/insolar/logicrunner/artifacts"
)

func InitializeContractMethods() map[string]XXX_insolar.ContractWrapper {
	return map[string]XXX_insolar.ContractWrapper{
		"account":         account.Initialize(),
		"costcenter":      costcenter.Initialize(),
		"deposit":         deposit.Initialize(),
		"member":          member.Initialize(),
		"migrationadmin":  migrationadmin.Initialize(),
		"migrationdaemon": migrationdaemon.Initialize(),
		"migrationshard":  migrationshard.Initialize(),
		"pkshard":         pkshard.Initialize(),
		"rootdomain":      rootdomain.Initialize(),
		"wallet":          wallet.Initialize(),
	}
}

func shouldLoadRef(strRef string) XXX_insolar.Reference {
	ref, err := XXX_insolar.NewReferenceFromString(strRef)
	if err != nil {
		panic(errors.Wrap(err, "Unexpected error, bailing out"))
	}
	return *ref
}

func InitializeCodeRefs() map[XXX_insolar.Reference]string {
	rv := make(map[XXX_insolar.Reference]string, 10)

	rv[shouldLoadRef("insolar:0AAABAtrxlP_Iiq10drn2FuNMs2VppatXni7MP5Iy47g.record")] = "account"
	rv[shouldLoadRef("insolar:0AAABAt3ka4Zhm241MIue3ibjyPHXE0GONYHMDtJEMEs.record")] = "costcenter"
	rv[shouldLoadRef("insolar:0AAABApWJDvbGfjDx2Qe8L-XfyFUZ1Ak-xcE6ViTULWw.record")] = "deposit"
	rv[shouldLoadRef("insolar:0AAABAoppTQrrSQt5rQ883tMp-IoLRJ-LwDloc-_WiFs.record")] = "member"
	rv[shouldLoadRef("insolar:0AAABAi0UBL8r3E8dtn66NJ-TcBoppzrRpp7JzKZOlLo.record")] = "migrationadmin"
	rv[shouldLoadRef("insolar:0AAABAq4jEiQHkJX-GKVM5pIQhUVtBPKWrV08Ycf85SY.record")] = "migrationdaemon"
	rv[shouldLoadRef("insolar:0AAABAi9NXoKZFkG1sIUjNtX1lLdr2v57Ej22q3SAEbw.record")] = "migrationshard"
	rv[shouldLoadRef("insolar:0AAABAhxOSY2jr3NGP38lV6vd97RpEyJYZuuBkwCcykA.record")] = "pkshard"
	rv[shouldLoadRef("insolar:0AAABAiprNXjHYYuFbiGWyHqOhVd1kiZcuVJruipVv7s.record")] = "rootdomain"
	rv[shouldLoadRef("insolar:0AAABAgNCLM5-bWKjwAzmla4KxnaQenrEahCeKXgwjOE.record")] = "wallet"

	return rv
}

func InitializePrototypeRefs() map[XXX_insolar.Reference]string {
	rv := make(map[XXX_insolar.Reference]string, 10)

	rv[shouldLoadRef("insolar:0AAABAijqpfzqLqOhivOFDQOK5OO_gW78OzTTniCChIU")] = "account"
	rv[shouldLoadRef("insolar:0AAABAiiIlRbDnHuBzCCo8E9V-kCUpb22kUkU2ebIsa8")] = "costcenter"
	rv[shouldLoadRef("insolar:0AAABAsPCPoB0_7TDBh7dydzcQcqFqlbDu0bDPGr27oY")] = "deposit"
	rv[shouldLoadRef("insolar:0AAABArZDDJnAoTN3EvlpVIvuANsDK7eBid_XU-qbZSU")] = "member"
	rv[shouldLoadRef("insolar:0AAABAv4b40_lF0ivLCNhzPcq1hKkHWpRSaZCfZuPDUU")] = "migrationadmin"
	rv[shouldLoadRef("insolar:0AAABAs7xI_AGLwMS4lHNeLrbXbog1tOZL4BQiV0FNLQ")] = "migrationdaemon"
	rv[shouldLoadRef("insolar:0AAABAp-wD4rEsoVt39uIJ6CdqepSCnt5xmwZcs4Twjw")] = "migrationshard"
	rv[shouldLoadRef("insolar:0AAABAiGN1L8F9gCH_keBaxOP4atp9fzLiIci7xOg-hs")] = "pkshard"
	rv[shouldLoadRef("insolar:0AAABAu9gOQ8PRiG_hT8l-hHXMXvc89IhJBemCzzAglQ")] = "rootdomain"
	rv[shouldLoadRef("insolar:0AAABAgfNy9VkTWQBamlz1DPbynRrVLzRtsRo-X2YI6U")] = "wallet"

	return rv
}

func InitializeCodeDescriptors() []XXX_artifacts.CodeDescriptor {
	rv := make([]XXX_artifacts.CodeDescriptor, 0, 10)

	// account
	rv = append(rv, XXX_artifacts.NewCodeDescriptor(
		/* code:        */ nil,
		/* machineType: */ XXX_insolar.MachineTypeBuiltin,
		/* ref:         */ shouldLoadRef("insolar:0AAABAtrxlP_Iiq10drn2FuNMs2VppatXni7MP5Iy47g.record"),
	))
	// costcenter
	rv = append(rv, XXX_artifacts.NewCodeDescriptor(
		/* code:        */ nil,
		/* machineType: */ XXX_insolar.MachineTypeBuiltin,
		/* ref:         */ shouldLoadRef("insolar:0AAABAt3ka4Zhm241MIue3ibjyPHXE0GONYHMDtJEMEs.record"),
	))
	// deposit
	rv = append(rv, XXX_artifacts.NewCodeDescriptor(
		/* code:        */ nil,
		/* machineType: */ XXX_insolar.MachineTypeBuiltin,
		/* ref:         */ shouldLoadRef("insolar:0AAABApWJDvbGfjDx2Qe8L-XfyFUZ1Ak-xcE6ViTULWw.record"),
	))
	// member
	rv = append(rv, XXX_artifacts.NewCodeDescriptor(
		/* code:        */ nil,
		/* machineType: */ XXX_insolar.MachineTypeBuiltin,
		/* ref:         */ shouldLoadRef("insolar:0AAABAoppTQrrSQt5rQ883tMp-IoLRJ-LwDloc-_WiFs.record"),
	))
	// migrationadmin
	rv = append(rv, XXX_artifacts.NewCodeDescriptor(
		/* code:        */ nil,
		/* machineType: */ XXX_insolar.MachineTypeBuiltin,
		/* ref:         */ shouldLoadRef("insolar:0AAABAi0UBL8r3E8dtn66NJ-TcBoppzrRpp7JzKZOlLo.record"),
	))
	// migrationdaemon
	rv = append(rv, XXX_artifacts.NewCodeDescriptor(
		/* code:        */ nil,
		/* machineType: */ XXX_insolar.MachineTypeBuiltin,
		/* ref:         */ shouldLoadRef("insolar:0AAABAq4jEiQHkJX-GKVM5pIQhUVtBPKWrV08Ycf85SY.record"),
	))
	// migrationshard
	rv = append(rv, XXX_artifacts.NewCodeDescriptor(
		/* code:        */ nil,
		/* machineType: */ XXX_insolar.MachineTypeBuiltin,
		/* ref:         */ shouldLoadRef("insolar:0AAABAi9NXoKZFkG1sIUjNtX1lLdr2v57Ej22q3SAEbw.record"),
	))
	// pkshard
	rv = append(rv, XXX_artifacts.NewCodeDescriptor(
		/* code:        */ nil,
		/* machineType: */ XXX_insolar.MachineTypeBuiltin,
		/* ref:         */ shouldLoadRef("insolar:0AAABAhxOSY2jr3NGP38lV6vd97RpEyJYZuuBkwCcykA.record"),
	))
	// rootdomain
	rv = append(rv, XXX_artifacts.NewCodeDescriptor(
		/* code:        */ nil,
		/* machineType: */ XXX_insolar.MachineTypeBuiltin,
		/* ref:         */ shouldLoadRef("insolar:0AAABAiprNXjHYYuFbiGWyHqOhVd1kiZcuVJruipVv7s.record"),
	))
	// wallet
	rv = append(rv, XXX_artifacts.NewCodeDescriptor(
		/* code:        */ nil,
		/* machineType: */ XXX_insolar.MachineTypeBuiltin,
		/* ref:         */ shouldLoadRef("insolar:0AAABAgNCLM5-bWKjwAzmla4KxnaQenrEahCeKXgwjOE.record"),
	))

	return rv
}

func InitializePrototypeDescriptors() []XXX_artifacts.PrototypeDescriptor {
	rv := make([]XXX_artifacts.PrototypeDescriptor, 0, 10)

	{ // account
		pRef := shouldLoadRef("insolar:0AAABAijqpfzqLqOhivOFDQOK5OO_gW78OzTTniCChIU")
		cRef := shouldLoadRef("insolar:0AAABAtrxlP_Iiq10drn2FuNMs2VppatXni7MP5Iy47g.record")
		rv = append(rv, XXX_artifacts.NewPrototypeDescriptor(
			/* head:         */ pRef,
			/* state:        */ *pRef.GetLocal(),
			/* code:         */ cRef,
		))
	}

	{ // costcenter
		pRef := shouldLoadRef("insolar:0AAABAiiIlRbDnHuBzCCo8E9V-kCUpb22kUkU2ebIsa8")
		cRef := shouldLoadRef("insolar:0AAABAt3ka4Zhm241MIue3ibjyPHXE0GONYHMDtJEMEs.record")
		rv = append(rv, XXX_artifacts.NewPrototypeDescriptor(
			/* head:         */ pRef,
			/* state:        */ *pRef.GetLocal(),
			/* code:         */ cRef,
		))
	}

	{ // deposit
		pRef := shouldLoadRef("insolar:0AAABAsPCPoB0_7TDBh7dydzcQcqFqlbDu0bDPGr27oY")
		cRef := shouldLoadRef("insolar:0AAABApWJDvbGfjDx2Qe8L-XfyFUZ1Ak-xcE6ViTULWw.record")
		rv = append(rv, XXX_artifacts.NewPrototypeDescriptor(
			/* head:         */ pRef,
			/* state:        */ *pRef.GetLocal(),
			/* code:         */ cRef,
		))
	}

	{ // member
		pRef := shouldLoadRef("insolar:0AAABArZDDJnAoTN3EvlpVIvuANsDK7eBid_XU-qbZSU")
		cRef := shouldLoadRef("insolar:0AAABAoppTQrrSQt5rQ883tMp-IoLRJ-LwDloc-_WiFs.record")
		rv = append(rv, XXX_artifacts.NewPrototypeDescriptor(
			/* head:         */ pRef,
			/* state:        */ *pRef.GetLocal(),
			/* code:         */ cRef,
		))
	}

	{ // migrationadmin
		pRef := shouldLoadRef("insolar:0AAABAv4b40_lF0ivLCNhzPcq1hKkHWpRSaZCfZuPDUU")
		cRef := shouldLoadRef("insolar:0AAABAi0UBL8r3E8dtn66NJ-TcBoppzrRpp7JzKZOlLo.record")
		rv = append(rv, XXX_artifacts.NewPrototypeDescriptor(
			/* head:         */ pRef,
			/* state:        */ *pRef.GetLocal(),
			/* code:         */ cRef,
		))
	}

	{ // migrationdaemon
		pRef := shouldLoadRef("insolar:0AAABAs7xI_AGLwMS4lHNeLrbXbog1tOZL4BQiV0FNLQ")
		cRef := shouldLoadRef("insolar:0AAABAq4jEiQHkJX-GKVM5pIQhUVtBPKWrV08Ycf85SY.record")
		rv = append(rv, XXX_artifacts.NewPrototypeDescriptor(
			/* head:         */ pRef,
			/* state:        */ *pRef.GetLocal(),
			/* code:         */ cRef,
		))
	}

	{ // migrationshard
		pRef := shouldLoadRef("insolar:0AAABAp-wD4rEsoVt39uIJ6CdqepSCnt5xmwZcs4Twjw")
		cRef := shouldLoadRef("insolar:0AAABAi9NXoKZFkG1sIUjNtX1lLdr2v57Ej22q3SAEbw.record")
		rv = append(rv, XXX_artifacts.NewPrototypeDescriptor(
			/* head:         */ pRef,
			/* state:        */ *pRef.GetLocal(),
			/* code:         */ cRef,
		))
	}

	{ // pkshard
		pRef := shouldLoadRef("insolar:0AAABAiGN1L8F9gCH_keBaxOP4atp9fzLiIci7xOg-hs")
		cRef := shouldLoadRef("insolar:0AAABAhxOSY2jr3NGP38lV6vd97RpEyJYZuuBkwCcykA.record")
		rv = append(rv, XXX_artifacts.NewPrototypeDescriptor(
			/* head:         */ pRef,
			/* state:        */ *pRef.GetLocal(),
			/* code:         */ cRef,
		))
	}

	{ // rootdomain
		pRef := shouldLoadRef("insolar:0AAABAu9gOQ8PRiG_hT8l-hHXMXvc89IhJBemCzzAglQ")
		cRef := shouldLoadRef("insolar:0AAABAiprNXjHYYuFbiGWyHqOhVd1kiZcuVJruipVv7s.record")
		rv = append(rv, XXX_artifacts.NewPrototypeDescriptor(
			/* head:         */ pRef,
			/* state:        */ *pRef.GetLocal(),
			/* code:         */ cRef,
		))
	}

	{ // wallet
		pRef := shouldLoadRef("insolar:0AAABAgfNy9VkTWQBamlz1DPbynRrVLzRtsRo-X2YI6U")
		cRef := shouldLoadRef("insolar:0AAABAgNCLM5-bWKjwAzmla4KxnaQenrEahCeKXgwjOE.record")
		rv = append(rv, XXX_artifacts.NewPrototypeDescriptor(
			/* head:         */ pRef,
			/* state:        */ *pRef.GetLocal(),
			/* code:         */ cRef,
		))
	}

	return rv
}
