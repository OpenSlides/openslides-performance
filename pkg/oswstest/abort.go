package oswstest

// hasAborted is a channel that is closed, when the testcases should be aborted
var hasAborted chan struct{}

// initialise the hasAborted channel.
func init() {
	hasAborted = make(chan struct{})
}

// Abort tries to stop all running testcases but still print some useful information
func Abort() {
	close(hasAborted)
}
