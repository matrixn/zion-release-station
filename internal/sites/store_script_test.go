package sites

import (
	"strings"
	"testing"
)

func TestEffectiveDeployScriptMigratesPersistedLegacyAtomicScript(t *testing.T) {
	legacyWithSpaces := "#!/bin/sh\nset -eu\n\n" +
		"SOURCE_DIR=\"${CURRENT_DIR:-${PROJECT_ROOT}/.current}\"\n" +
		"TARGET_DIR=\"${WEB_ROOT:?WEB_ROOT is required}\"\n" +
		"RELEASE_ID=\"${RELEASE_ID:-manual}\"\n" +
		"TARGET_PARENT=$(dirname \"$TARGET_DIR\")\n" +
		"TARGET_NAME=$(basename \"$TARGET_DIR\")\n" +
		"STAGING_DIR=\"${TARGET_PARENT}/.${TARGET_NAME}.staging-${RELEASE_ID}\"\n" +
		"BACKUP_DIR=\"${TARGET_PARENT}/.${TARGET_NAME}.previous-${RELEASE_ID}\"\n\n" +
		"mv \"$STAGING_DIR\" \"$TARGET_DIR\"\n"

	if got := EffectiveDeployScript("atomic", legacyWithSpaces); got != DefaultAtomicDeployScript {
		t.Fatalf("legacy atomic script was not migrated")
	}
}

func TestEffectiveDeployScriptKeepsCustomAtomicScript(t *testing.T) {
	custom := "#!/bin/sh\nset -eu\necho custom deploy\n"
	if got := EffectiveDeployScript("atomic", custom); got != strings.TrimSpace(custom) {
		t.Fatalf("custom atomic script was unexpectedly replaced: %q", got)
	}
}

func TestEffectiveDeployScriptMigratesPreviousGeneratedDefault(t *testing.T) {
	previousGenerated := `#!/bin/sh
set -eu
SOURCE_DIR="${CURRENT_DIR:-${PROJECT_ROOT}/.current}"
TARGET_DIR="${WEB_ROOT:-${PROJECT_ROOT:?PROJECT_ROOT is required}}"
RELEASE_ID="${RELEASE_ID:-manual}"
STATE_DIR="${PROJECT_ROOT:?PROJECT_ROOT is required}/.zion"
STAGING_DIR="${STATE_DIR}/document-root-staging-${RELEASE_ID}"
CONTENT_DIR="$SOURCE_DIR"
cp -a "$CONTENT_DIR"/. "$STAGING_DIR"/
`

	if got := EffectiveDeployScript("atomic", previousGenerated); got != DefaultAtomicDeployScript {
		t.Fatalf("previous generated default was not migrated")
	}
}

func TestDefaultAtomicScriptBuildsComposerProjectsAndRunsAvailableTests(t *testing.T) {
	for _, fragment := range []string{
		`if [ -f "$CONTENT_DIR/composer.json" ]; then`,
		`install --no-interaction --prefer-dist --optimize-autoloader`,
		`run-script --list`,
		`run-script test --no-interaction`,
		`vendor/bin/phpunit`,
		`vendor/bin/pest`,
	} {
		if !strings.Contains(DefaultAtomicDeployScript, fragment) {
			t.Fatalf("default script does not contain Composer step %q", fragment)
		}
	}
}
