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
