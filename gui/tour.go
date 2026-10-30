package gui

/*
IsNewUser tells the page whether this is a fresh install, so it offers the new user tour after the caveat warning. It waits for startup, which decides it.

Returns:
  - bool: true when neither settings.json nor the cache existed at startup
*/
func (a *App) IsNewUser() bool {
	a.waitReady()
	return a.newUser
}
