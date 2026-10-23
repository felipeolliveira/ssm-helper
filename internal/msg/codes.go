package msg

// Message codes. Every code needs a text in each frontend dictionary
// (frontend/src/i18n/locales/*.json, under "messages"); `make test` checks it.
// Parameters are listed next to each code.
const (
	UnexpectedError = "UNEXPECTED_ERROR" // detail: the error text

	// Files
	JSONInvalid       = "JSON_INVALID"         // detail: parser error
	JSONRootNotObject = "JSON_ROOT_NOT_OBJECT" //

	// Environment validation
	EnvFieldRequired   = "ENV_FIELD_REQUIRED"    // path
	EnvSectionMissing  = "ENV_SECTION_MISSING"   // section
	EnvNoProfiles      = "ENV_NO_PROFILES"       //
	EnvNoTargets       = "ENV_NO_TARGETS"        //
	EnvInvalidName     = "ENV_INVALID_NAME"      // path
	EnvProfileNotFound = "ENV_PROFILE_NOT_FOUND" // path, profile
	EnvInvalidDomain   = "ENV_INVALID_DOMAIN"    // path, value
	EnvInvalidURL      = "ENV_INVALID_URL"       // path, value

	// Environment files and actions
	EnvNotFound              = "ENV_NOT_FOUND"               //
	EnvDoesNotExist          = "ENV_DOES_NOT_EXIST"          // name
	EnvNewNameInvalid        = "ENV_NEW_NAME_INVALID"        //
	EnvNameTaken             = "ENV_NAME_TAKEN"              // name
	EnvCopyFailed            = "ENV_COPY_FAILED"             // detail
	EnvRenameCurrentRequired = "ENV_RENAME_CURRENT_REQUIRED" //
	EnvRenameSameName        = "ENV_RENAME_SAME_NAME"        //
	EnvRenameHasTunnels      = "ENV_RENAME_HAS_TUNNELS"      //
	EnvNotReady              = "ENV_NOT_READY"               //
	EnvSaved                 = "ENV_SAVED"                   // file
	EnvCreated               = "ENV_CREATED"                 // file
	EnvRenamed               = "ENV_RENAMED"                 // from, to
	EnvMigrated              = "ENV_MIGRATED"                // from, to
	EnvMigrationFailed       = "ENV_MIGRATION_FAILED"        // file (may be empty); detail
	ProfileNotFound          = "PROFILE_NOT_FOUND"           // profile
	TargetNotFound           = "TARGET_NOT_FOUND"            // target

	// Palettes (ssm-helper.json)
	PaletteAppearanceNotObject = "PALETTE_APPEARANCE_NOT_OBJECT" //
	PaletteListNotObject       = "PALETTE_LIST_NOT_OBJECT"       //
	PaletteReservedName        = "PALETTE_RESERVED_NAME"         // path
	PaletteInvalidName         = "PALETTE_INVALID_NAME"          // path
	PaletteUnknownColor        = "PALETTE_UNKNOWN_COLOR"         // path
	PaletteInvalidColor        = "PALETTE_INVALID_COLOR"         // path
	PaletteNotFound            = "PALETTE_NOT_FOUND"             // palette

	// Settings (ssm-helper.json)
	SettingsReadFailed   = "SETTINGS_READ_FAILED"   // file; detail
	SettingsDefaultsUsed = "SETTINGS_DEFAULTS_USED" // file
	SettingsSaveFailed   = "SETTINGS_SAVE_FAILED"   // file; detail
	SettingsMigrated     = "SETTINGS_MIGRATED"      // from, to
	AppearanceMigrated   = "APPEARANCE_MIGRATED"    // from, to
	AppearanceSaved      = "APPEARANCE_SAVED"       //

	// SSO
	SSOLoginExpired          = "SSO_LOGIN_EXPIRED"           //
	SSORegisterFailed        = "SSO_REGISTER_FAILED"         // detail
	SSOStartFailed           = "SSO_START_FAILED"            // detail
	SSOCodeExpired           = "SSO_CODE_EXPIRED"            //
	SSOLoginFailed           = "SSO_LOGIN_FAILED"            // detail
	SSORoleCredentialsFailed = "SSO_ROLE_CREDENTIALS_FAILED" // detail
	SSOVerifyFailed          = "SSO_VERIFY_FAILED"           // role; detail
	SSOLoginInProgress       = "SSO_LOGIN_IN_PROGRESS"       // profile
	SSOLoginStarting         = "SSO_LOGIN_STARTING"          //
	SSOLoginSucceeded        = "SSO_LOGIN_SUCCEEDED"         // profile

	// Tunnels
	TunnelStartFailed      = "TUNNEL_START_FAILED"       // detail
	TunnelProcessFailed    = "TUNNEL_PROCESS_FAILED"     // detail
	TunnelPortInUse        = "TUNNEL_PORT_IN_USE"        // port; detail
	TunnelAlreadyActive    = "TUNNEL_ALREADY_ACTIVE"     // target
	TunnelCheckingSession  = "TUNNEL_CHECKING_SESSION"   //
	TunnelLoginRequired    = "TUNNEL_LOGIN_REQUIRED"     //
	TunnelStarting         = "TUNNEL_STARTING"           //
	TunnelActive           = "TUNNEL_ACTIVE"             // port
	TunnelEndedBeforeReady = "TUNNEL_ENDED_BEFORE_READY" // detail: last plugin output
	TunnelEndedWithError   = "TUNNEL_ENDED_WITH_ERROR"   // detail
	TunnelClosed           = "TUNNEL_CLOSED"             //
	TunnelCancelled        = "TUNNEL_CANCELLED"          //
	TunnelPluginOutput     = "TUNNEL_PLUGIN_OUTPUT"      // detail: a line from the AWS plugin, in English

	// Chrome
	ChromeNotFound   = "CHROME_NOT_FOUND"   //
	ChromeOpenFailed = "CHROME_OPEN_FAILED" // detail
	ChromeCommand    = "CHROME_COMMAND"     // detail: the command line used
	ChromeOpened     = "CHROME_OPENED"      //

	// Data folder (ssm-helper-data)
	DataDirCreateFailed = "DATA_DIR_CREATE_FAILED" // dir; detail
	DataDirNotProtected = "DATA_DIR_NOT_PROTECTED" // dir; detail
	DataLegacyDirUnused = "DATA_LEGACY_DIR_UNUSED" // legacy, dir
)
