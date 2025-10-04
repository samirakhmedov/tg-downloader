package logger

import "go.uber.org/fx/fxevent"

// FxLogger adapts our custom Logger to the FX event logger interface
type FxLogger struct {
	logger *Logger
}

func (f *FxLogger) LogEvent(event fxevent.Event) {
	switch e := event.(type) {
	case *fxevent.OnStartExecuting:
		f.logger.Debug("OnStart hook executing: " + e.FunctionName)
	case *fxevent.OnStartExecuted:
		if e.Err != nil {
			f.logger.Error("OnStart hook failed: " + e.FunctionName + " - " + e.Err.Error())
		} else {
			f.logger.Debug("OnStart hook executed: " + e.FunctionName)
		}
	case *fxevent.OnStopExecuting:
		f.logger.Debug("OnStop hook executing: " + e.FunctionName)
	case *fxevent.OnStopExecuted:
		if e.Err != nil {
			f.logger.Error("OnStop hook failed: " + e.FunctionName + " - " + e.Err.Error())
		} else {
			f.logger.Debug("OnStop hook executed: " + e.FunctionName)
		}
	case *fxevent.Supplied:
		f.logger.Debug("Supplied: " + e.TypeName)
	case *fxevent.Provided:
		f.logger.Debug("Provided: " + e.OutputTypeNames[0])
	case *fxevent.Invoking:
		f.logger.Debug("Invoking: " + e.FunctionName)
	case *fxevent.Invoked:
		if e.Err != nil {
			f.logger.Error("Invoke failed: " + e.FunctionName + " - " + e.Err.Error())
		}
	case *fxevent.Stopping:
		f.logger.Info("Stopping application")
	case *fxevent.Stopped:
		if e.Err != nil {
			f.logger.Error("Stop failed: " + e.Err.Error())
		}
	case *fxevent.RollingBack:
		f.logger.Error("Rolling back: " + e.StartErr.Error())
	case *fxevent.RolledBack:
		if e.Err != nil {
			f.logger.Error("Rollback failed: " + e.Err.Error())
		}
	case *fxevent.Started:
		if e.Err != nil {
			f.logger.Error("Start failed: " + e.Err.Error())
		} else {
			f.logger.Info("Application started")
		}
	case *fxevent.LoggerInitialized:
		if e.Err != nil {
			f.logger.Error("Logger initialization failed: " + e.Err.Error())
		}
	}
}
