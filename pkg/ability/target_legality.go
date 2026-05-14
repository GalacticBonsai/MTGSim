package ability

func (ee *ExecutionEngine) isTargetStillLegal(target any, targetReq Target, controller AbilityPlayer) bool {
	if ee == nil {
		return false
	}
	if targetReq.Enhanced != nil {
		return ee.targetValidator.ValidateTarget(target, *targetReq.Enhanced, controller).IsLegal
	}
	return ee.isValidBasicTarget(target, targetReq)
}
