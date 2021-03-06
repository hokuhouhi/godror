// Copyright 2018, 2020 The Godror Authors
//
//
// SPDX-License-Identifier: UPL-1.0 OR Apache-2.0

package godror_test

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	errors "golang.org/x/xerrors"

	godror "github.com/godror/godror"
)

func TestHeterogeneousPoolIntegration(t *testing.T) {

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	const proxyPassword = "myPassword666myPassword"
	const proxyUser = "test_proxyUser"

	cs, err := godror.ParseConnString(testConStr)
	if err != nil {
		t.Fatal(err)
	}
	cs.Heterogeneous = true
	username := cs.UserName
	testHeterogeneousConStr := cs.StringWithPassword()
	t.Log(testHeterogeneousConStr)

	var testHeterogeneousDB *sql.DB
	if testHeterogeneousDB, err = sql.Open("godror", testHeterogeneousConStr); err != nil {
		t.Fatal(errors.Errorf("%s: %w", testHeterogeneousConStr, err))
	}
	defer testHeterogeneousDB.Close()
	testHeterogeneousDB.SetMaxIdleConns(0)

	// Check that it works
	conn, err := testHeterogeneousDB.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	conn.ExecContext(ctx, `ALTER SESSION SET "_ORACLE_SCRIPT"=true`)
	conn.ExecContext(ctx, fmt.Sprintf("DROP USER %s", proxyUser))

	for _, qry := range []string{
		fmt.Sprintf("CREATE USER %s IDENTIFIED BY "+proxyPassword, proxyUser),
		fmt.Sprintf("GRANT CREATE SESSION TO %s", proxyUser),
		fmt.Sprintf("ALTER USER %s GRANT CONNECT THROUGH %s", proxyUser, username),
	} {
		if _, err := conn.ExecContext(ctx, qry); err != nil {
			if strings.Contains(err.Error(), "ORA-01031:") {
				t.Log("Please issue this:\nGRANT CREATE USER, DROP USER, ALTER USER TO " + username + ";\n" +
					"GRANT CREATE SESSION TO " + username + " WITH ADMIN OPTION;\n")
			}
			t.Skip(errors.Errorf("%s: %w", qry, err))
		}
	}
	defer func() { testHeterogeneousDB.ExecContext(context.Background(), "DROP USER "+proxyUser) }()

	for tName, tCase := range map[string]struct {
		In   context.Context
		Want string
	}{
		"noContext": {In: ctx, Want: username},
		"proxyUser": {In: godror.ContextWithUserPassw(ctx, proxyUser, proxyPassword, ""), Want: proxyUser},
	} {
		t.Run(tName, func(t *testing.T) {
			var result string
			if err = testHeterogeneousDB.QueryRowContext(tCase.In, "SELECT user FROM dual").Scan(&result); err != nil {
				t.Fatal(err)
			}
			if !strings.EqualFold(tCase.Want, result) {
				t.Errorf("%s: currentUser got %s, wanted %s", tName, result, tCase.Want)
			}
		})

	}

}

func TestContextWithUserPassw(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cs, err := godror.ParseConnString(testConStr)
	if err != nil {
		t.Fatal(err)
	}
	cs.Heterogeneous = true
	username, password := cs.UserName, cs.Password
	cs.UserName, cs.Password = "", ""
	testHeterogeneousConStr := cs.StringWithPassword()
	t.Log(testConStr, " -> ", testHeterogeneousConStr)

	var testHeterogeneousDB *sql.DB
	if testHeterogeneousDB, err = sql.Open("godror", testHeterogeneousConStr); err != nil {
		t.Fatal(errors.Errorf("%s: %w", testHeterogeneousConStr, err))
	}
	defer testHeterogeneousDB.Close()

	ctx = godror.ContextWithUserPassw(ctx, username, password, "")
	if err := testHeterogeneousDB.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
	t.Log(ctx)
}
