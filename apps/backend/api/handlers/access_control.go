package handlers

// Legacy access-control handler implementations were retired after the module
// extraction. The active HTTP entrypoints now live in access_control_module.go
// and delegate to apps/backend/modules/accesscontrol while keeping routes and
// payloads stable.
