package palworld

// assertGameNotRunning returns an error if Palworld is running (its save files
// would be locked / mid-write).
func assertGameNotRunning() error {
	ids, err := processIDsOS("Palworld")
	if err == nil && len(ids) > 0 {
		return errGameRunning
	}
	return nil
}

var errGameRunning = errStr("Palworld is running; please close it before modifying saves")
