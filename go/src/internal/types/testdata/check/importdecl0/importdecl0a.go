// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package importdecl0

import (
	// we can have multiple blank imports (was bug)
	init "fmt" /* ERROR "cannot import package as init" */
	_ "math"
	_ "net/rpc"
	// reflect defines a type "flag" which shows up in the gc export data
	"reflect"
	. "reflect" /* ERROR "imported and not used" */
)

import "math" /* ERROR "imported and not used" */
import m /* ERROR "imported as m and not used" */ "math"
import _ "math"

import (
	"math/big" /* ERROR "imported and not used" */
	_ "math/big"
	b "math/big" /* ERROR "imported as b and not used" */
)

import "fmt"
import f1 "fmt"
import f2 "fmt"

// reflect.flag must not be visible in this package
type flag int
type _ reflect.flag /* ERROR "name flag not exported by package reflect" */

// imported package name may conflict with local objects
type reflect /* ERROR "reflect already declared" */ int

// dot-imported exported objects may conflict with local objects
type Value /* ERROR "Value already declared through dot-import of package reflect" */ struct{}

var _ = fmt.Println // use "fmt"

func _() {
	f1.Println() // use "fmt"
}

func _() {
	_ = func() {
		f2.Println() // use "fmt"
	}
}
