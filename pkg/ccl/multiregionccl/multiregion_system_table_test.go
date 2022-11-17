// Copyright 2022 The Cockroach Authors.
//
// Licensed as a CockroachDB Enterprise file under the Cockroach Community
// License (the "License"); you may not use this file except in compliance with
// the License. You may obtain a copy of the License at
//
//     https://github.com/cockroachdb/cockroach/blob/master/licenses/CCL.txt

package multiregionccl

import (
	"context"
	gosql "database/sql"
	"testing"

	"github.com/cockroachdb/apd/v3"
	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/ccl/multiregionccl/multiregionccltestutils"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/settings/cluster"
	"github.com/cockroachdb/cockroach/pkg/sql"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descpb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descs"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/tabledesc"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/typedesc"
	"github.com/cockroachdb/cockroach/pkg/sql/enum"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/sqlliveness"
	"github.com/cockroachdb/cockroach/pkg/sql/sqlliveness/slstorage"
	"github.com/cockroachdb/cockroach/pkg/sql/types"
	"github.com/cockroachdb/cockroach/pkg/testutils/serverutils"
	"github.com/cockroachdb/cockroach/pkg/testutils/sqlutils"
	"github.com/cockroachdb/cockroach/pkg/util/envutil"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/stretchr/testify/require"
)


func TestMrSystemDatabase(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)
	defer envutil.TestSetEnv(t, "COCKROACH_MR_SYSTEM_DATABASE", "1")()

	// Enable settings required for configuring a tenant's system database as multi-region.
	cs := cluster.MakeTestingClusterSettings()
	sql.SecondaryTenantsMultiRegionAbstractionsEnabled.Override(context.Background(), &cs.SV, true)
	sql.SecondaryTenantZoneConfigsEnabled.Override(context.Background(), &cs.SV, true)

	cluster, _, cleanup := multiregionccltestutils.TestingCreateMultiRegionCluster(t, 3, base.TestingKnobs{}, multiregionccltestutils.WithSettings(cs))
	defer cleanup()

	id, err := roachpb.MakeTenantID(11)
	require.NoError(t, err)

	tenantArgs := base.TestTenantArgs{
		Settings: cs,
		TenantID: id,
		Locality: *cluster.Servers[0].Locality(),
	}
	_, tenantSQL := serverutils.StartTenant(t, cluster.Servers[0], tenantArgs)

	tDB := sqlutils.MakeSQLRunner(tenantSQL)

	tDB.Exec(t, `ALTER DATABASE system SET PRIMARY REGION "us-east1"`)
	tDB.Exec(t, `ALTER DATABASE system ADD REGION "us-east2"`)
	tDB.Exec(t, `ALTER DATABASE system ADD REGION "us-east3"`)

	tDB.Exec(t, `SELECT crdb_internal.unsafe_optimize_system_database()`)

	// Run schema validations to ensure the manual descriptor modifications are
	// okay.
	tDB.CheckQueryResults(t, `SELECT * FROM crdb_internal.invalid_objects`, [][]string{})

	t.Run("Sqlliveness", func(t *testing.T) {
		row := tDB.QueryRow(t, `SELECT crdb_region, session_uuid, expiration FROM system.sqlliveness LIMIT 1`)
		var sessionUUID string
		var crdbRegion string
		var rawExpiration apd.Decimal
		row.Scan(&crdbRegion, &sessionUUID, &rawExpiration)
		require.Equal(t, "us-east1", crdbRegion)
	})

	t.Run("Sqlinstances", func(t *testing.T) {
		t.Run("InUse", func(t *testing.T) {
			query := `
                SELECT id, addr, session_id, locality, crdb_region
                FROM system.sql_instances
                WHERE session_id IS NOT NULL
            `
			rows := tDB.Query(t, query)
			require.True(t, rows.Next())
			for {
				var id base.SQLInstanceID
				var addr, locality string
				var crdb_region string
				var session sqlliveness.SessionID

				require.NoError(t, rows.Scan(&id, &addr, &session, &locality, &crdb_region))

				require.True(t, 0 < id)
				require.NotEmpty(t, addr)
				require.NotEmpty(t, locality)
				require.NotEmpty(t, session)
				require.NotEmpty(t, crdb_region)

				require.Equal(t, "us-east1", crdb_region)

				if !rows.Next() {
					break
				}
			}
			require.NoError(t, rows.Close())
		})

		t.Run("Preallocated", func(t *testing.T) {
			query := `
                SELECT id, addr, session_id, locality, crdb_region
                FROM system.sql_instances
                WHERE session_id IS NULL
            `
			rows := tDB.Query(t, query)
			require.True(t, rows.Next())
			for {
				var id base.SQLInstanceID
				var addr, locality, session gosql.NullString
				var crdb_region string

				require.NoError(t, rows.Scan(&id, &addr, &session, &locality, &crdb_region))

				require.True(t, 0 < id)
				require.False(t, addr.Valid)
				require.False(t, locality.Valid)
				require.False(t, session.Valid)
				require.NotEmpty(t, crdb_region)

				if !rows.Next() {
					break
				}
			}
			require.NoError(t, rows.Close())
		})
	})
}

func TestTenantStartupWithMultiRegionEnum(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)
	defer envutil.TestSetEnv(t, "COCKROACH_MR_SYSTEM_DATABASE", "1")()

	// Enable settings required for configuring a tenant's system database as multi-region.
	cs := cluster.MakeTestingClusterSettings()
	sql.SecondaryTenantsMultiRegionAbstractionsEnabled.Override(context.Background(), &cs.SV, true)
	sql.SecondaryTenantZoneConfigsEnabled.Override(context.Background(), &cs.SV, true)

	tc, _, cleanup := multiregionccltestutils.TestingCreateMultiRegionCluster(
		t, 3 /*numServers*/, base.TestingKnobs{}, multiregionccltestutils.WithSettings(cs),
	)
	defer cleanup()

	tenID := roachpb.MustMakeTenantID(10)
	ten, tSQL := serverutils.StartTenant(t, tc.Server(0), base.TestTenantArgs{
		Settings: cs,
		TenantID: tenID,
		Locality: roachpb.Locality{
			Tiers: []roachpb.Tier{
				{Key: "region", Value: "us-east1"},
			},
		},
	})
	defer tSQL.Close()
	tenSQLDB := sqlutils.MakeSQLRunner(tSQL)

	// Update system database with regions.
	tenSQLDB.Exec(t, `ALTER DATABASE system SET PRIMARY REGION "us-east1"`)
	tenSQLDB.Exec(t, `ALTER DATABASE system ADD REGION "us-east2"`)
	tenSQLDB.Exec(t, `ALTER DATABASE system ADD REGION "us-east3"`)

	ten2, tSQL2 := serverutils.StartTenant(t, tc.Server(2), base.TestTenantArgs{
		Settings: cs,
		TenantID: tenID,
		Existing: true,
		Locality: roachpb.Locality{
			Tiers: []roachpb.Tier{
				{Key: "region", Value: "us-east3"},
			},
		},
	})
	defer tSQL2.Close()
	tenSQLDB2 := sqlutils.MakeSQLRunner(tSQL2)

	// The sqlliveness entry created by the first SQL server has enum.One as the
	// region as the system database hasn't been updated when it first started.
	var sessionID string
	tenSQLDB2.QueryRow(t, `SELECT session_id FROM system.sql_instances WHERE id = $1`,
		ten.SQLInstanceID()).Scan(&sessionID)
	region, id, err := slstorage.UnsafeDecodeSessionID(sqlliveness.SessionID(sessionID))
	require.NoError(t, err)
	require.NotNil(t, id)
	require.Equal(t, enum.One, region)

	// Ensure that the sqlliveness entry created by the second SQL server has
	// the right region and session UUID.
	tenSQLDB2.QueryRow(t, `SELECT session_id FROM system.sql_instances WHERE id = $1`,
		ten2.SQLInstanceID()).Scan(&sessionID)
	region, id, err = slstorage.UnsafeDecodeSessionID(sqlliveness.SessionID(sessionID))
	require.NoError(t, err)
	require.NotNil(t, id)
	require.NotEqual(t, enum.One, region)

	rows := tenSQLDB2.Query(t, `SELECT crdb_region, session_uuid FROM system.sqlliveness`)
	defer rows.Close()
	livenessMap := map[string][]byte{}
	for rows.Next() {
		var region, sessionUUID string
		require.NoError(t, rows.Scan(&region, &sessionUUID))
		livenessMap[sessionUUID] = []byte(region)
	}
	require.NoError(t, rows.Err())
	r, ok := livenessMap[string(id)]
	require.True(t, ok)
	require.Equal(t, r, region)
}
