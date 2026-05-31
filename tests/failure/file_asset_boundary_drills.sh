#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

echo "drill=file_quota_excludes_expired_assets"
(cd "${ROOT_DIR}" && go test ./internal/task -run 'TestFile(QuotaExcludesExpiredAssets|ServiceCleanupExpiredFiles|ServiceURLRegistrationDoesNotCreateGatewayFileURL)' -count=1)

echo "drill=file_base64_size_and_stream_disabled_contract"
(cd "${ROOT_DIR}" && go test ./internal/dataplane/parser -run 'TestParserFile(Base64|Base64RejectsOversizedDecodedInput|StreamReturnsFeatureNotEnabled)' -count=1)

echo "drill=file_asset_cleanup_job"
(cd "${ROOT_DIR}" && go test ./internal/worker/jobs -run TestFileAssetCleaner -count=1)

echo "drill=file_asset_contract"
(cd "${ROOT_DIR}" && go test ./tests/contract -run 'TestMedia(FileObjectIsTransientInputAsset|URLFileObjectDoesNotExposeGatewayHostedURL)' -count=1)

echo "file_asset_boundary_drills=passed"
