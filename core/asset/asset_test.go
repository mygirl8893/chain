package asset

import (
	"context"
	"encoding/hex"
	"reflect"
	"testing"

	"chain/core/signers"
	"chain/database/pg"
	"chain/database/pg/pgtest"
	"chain/errors"
	"chain/protocol/bc"
	"chain/testutil"
)

func TestDefineAsset(t *testing.T) {
	ctx := pg.NewContext(context.Background(), pgtest.NewTx(t))
	keys := []string{testutil.TestXPub.String()}
	var initialBlockHash bc.Hash
	asset, err := Define(ctx, keys, 1, nil, initialBlockHash, "", nil, nil)
	if err != nil {
		testutil.FatalErr(t, err)
	}
	if asset.sortID == "" {
		t.Error("asset.sortID empty")
	}

	// Verify that the asset was defined.
	var id string
	var checkQ = `SELECT id FROM assets`
	err = pg.QueryRow(ctx, checkQ).Scan(&id)
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}
	if id != asset.AssetID.String() {
		t.Errorf("expected new asset %s to be recorded as %s", asset.AssetID.String(), id)
	}
}

func TestDefineAssetIdempotency(t *testing.T) {
	dbtx := pgtest.NewTx(t)
	ctx := pg.NewContext(context.Background(), dbtx)
	token := "test_token"
	keys := []string{testutil.TestXPub.String()}
	var initialBlockHash bc.Hash
	asset0, err := Define(ctx, keys, 1, nil, initialBlockHash, "", nil, &token)
	if err != nil {
		testutil.FatalErr(t, err)
	}

	asset1, err := Define(ctx, keys, 1, nil, initialBlockHash, "", nil, &token)
	if err != nil {
		testutil.FatalErr(t, err)
	}

	// asset0 and asset1 should be exactly the same because they use the same client token
	if !reflect.DeepEqual(asset0, asset1) {
		t.Errorf("expected %v and %v to match", asset0, asset1)
	}
}

func TestSetAssetTags(t *testing.T) {
	dbtx := pgtest.NewTx(t)
	ctx := pg.NewContext(context.Background(), dbtx)
	keys := []string{testutil.TestXPub.String()}
	var initialBlockHash bc.Hash
	asset, err := Define(ctx, keys, 1, nil, initialBlockHash, "some-alias", nil, nil)
	if err != nil {
		testutil.FatalErr(t, err)
	}

	newTags := map[string]interface{}{"someTag": "taggityTag"}

	// first set by ID
	updated, err := SetTags(ctx, asset.AssetID, "", newTags)
	if err != nil {
		testutil.FatalErr(t, err)
	}

	asset.Tags = newTags
	if !reflect.DeepEqual(asset, updated) {
		t.Errorf("got = %+v want %+v", updated, asset)
	}

	// now set by alias
	newTags = map[string]interface{}{"someTag": "alias-alias"}
	updated, err = SetTags(ctx, bc.AssetID{}, "some-alias", newTags)
	if err != nil {
		testutil.FatalErr(t, err)
	}

	asset.Tags = newTags
	if !reflect.DeepEqual(asset, updated) {
		t.Errorf("got = %+v want %+v", updated, asset)
	}
}

func TestSetNonLocalAssetTags(t *testing.T) {
	dbtx := pgtest.NewTx(t)
	ctx := pg.NewContext(context.Background(), dbtx)
	newTags := map[string]interface{}{"someTag": "taggityTag"}
	assetID := mustDecodeAssetID("2d194241795a28af3345ffcc64fd31d8819c56f4c4d4b4360763a259152aa393")

	updated, err := SetTags(ctx, assetID, "", newTags)
	if err != nil {
		testutil.FatalErr(t, err)
	}

	want := &Asset{
		AssetID: assetID,
		Tags:    newTags,
	}

	if !reflect.DeepEqual(updated, want) {
		t.Errorf("got = %+v want %+v", updated, want)
	}
}

func TestDefineAndArchiveAssetByID(t *testing.T) {
	dbtx := pgtest.NewTx(t)
	ctx := pg.NewContext(context.Background(), dbtx)
	keys := []string{testutil.TestXPub.String()}
	var initialBlockHash bc.Hash
	asset, err := Define(ctx, keys, 1, nil, initialBlockHash, "", nil, nil)
	if err != nil {
		testutil.FatalErr(t, err)
	}

	err = Archive(ctx, asset.AssetID, "")
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}

	// Verify that the asset was archived.
	_, err = FindByID(ctx, asset.AssetID)
	if err != ErrArchived {
		t.Error("expected asset id to be archived")
	}

	// Verify that the signer was archived.
	_, err = signers.Find(ctx, "asset", asset.Signer.ID)
	if errors.Root(err) != signers.ErrArchived {
		t.Error("expected signer to be archived")
	}
}

func TestDefineAndArchiveAssetByAlias(t *testing.T) {
	dbtx := pgtest.NewTx(t)
	ctx := pg.NewContext(context.Background(), dbtx)
	keys := []string{testutil.TestXPub.String()}
	var initialBlockHash bc.Hash
	asset, err := Define(ctx, keys, 1, nil, initialBlockHash, "some-alias", nil, nil)
	if err != nil {
		testutil.FatalErr(t, err)
	}

	err = Archive(ctx, bc.AssetID{}, "some-alias")
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}

	// Verify that the asset was archived.
	_, err = assetByAlias(ctx, "some-alias")
	if err != ErrArchived {
		t.Error("expected asset id to be archived")
	}

	// Verify that the signer was archived.
	_, err = signers.Find(ctx, "asset", asset.Signer.ID)
	if errors.Root(err) != signers.ErrArchived {
		t.Error("expected signer to be archived")
	}
}

func TestFindAssetByID(t *testing.T) {
	dbtx := pgtest.NewTx(t)
	ctx := pg.NewContext(context.Background(), dbtx)
	keys := []string{testutil.TestXPub.String()}
	var initialBlockHash bc.Hash
	asset, err := Define(ctx, keys, 1, nil, initialBlockHash, "", nil, nil)
	if err != nil {
		testutil.FatalErr(t, err)
	}

	found, err := FindByID(ctx, asset.AssetID)
	if err != nil {
		testutil.FatalErr(t, err)
	}

	if !reflect.DeepEqual(asset, found) {
		t.Errorf("expected %v and %v to match", asset, found)
	}
}

func TestAssetByClientToken(t *testing.T) {
	dbtx := pgtest.NewTx(t)
	ctx := pg.NewContext(context.Background(), dbtx)
	keys := []string{testutil.TestXPub.String()}
	token := "test_token"
	var initialBlockHash bc.Hash

	asset, err := Define(ctx, keys, 1, nil, initialBlockHash, "", nil, &token)
	if err != nil {
		testutil.FatalErr(t, err)
	}

	found, err := assetByClientToken(ctx, token)
	if err != nil {
		testutil.FatalErr(t, err)
	}

	if found.AssetID != asset.AssetID {
		t.Fatalf("assetByClientToken(\"test_token\")=%x, want %x", found.AssetID[:], asset.AssetID[:])
	}
}

func mustDecodeAssetID(hash string) bc.AssetID {
	var h bc.AssetID
	if len(hash) != hex.EncodedLen(len(h)) {
		panic("wrong length hash")
	}
	_, err := hex.Decode(h[:], []byte(hash))
	if err != nil {
		panic(err)
	}
	return h
}
