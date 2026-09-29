package scene

import (
	"fmt"
	"math"
)

// frameAlignmentTolerance 是判定「时长 × 帧率」为整数时允许的帧误差。
//
// 取 1/1000 帧：精确到 6 位小数的 N/fps 时长（如 14.366667）误差约 1e-5 帧，
// 能通过；3 位小数的近似值（如 14.367 → 431.01 帧）会被拦下。
const frameAlignmentTolerance = 1e-3

// VerifyFrameAlignment 要求镜头时长正好是整数帧。
//
// 渲染器按 ceil(时长 × fps) 出帧：14.367 秒在 30fps 下是 431.01 帧，会渲出
// 432 帧，镜头比时间轴长一帧，拼接后整片与配音错开。这类错误是确定性的，
// 重跑只会得到同样的产物，所以返回 VerificationError 走不重试的路径。
func VerifyFrameAlignment(s Scene) error {
	canvas, err := CanvasOf(s.Directory, s.StyleGuide)
	if err != nil {
		return err
	}
	fps := float64(canvas.FPS)
	frames := s.DurationSeconds * fps
	if math.Abs(frames-math.Round(frames)) <= frameAlignmentTolerance {
		return nil
	}
	lower := math.Floor(frames)
	upper := math.Ceil(frames)
	return &VerificationError{Output: s.Output, Problems: []string{fmt.Sprintf(
		"镜头 %s 时长 %g 秒 × %dfps = %.2f 帧，不是整数帧；渲染器会向上取整多出一帧。"+
			"请改成 %.6f 秒（%d 帧）或 %.6f 秒（%d 帧），并同步调整相邻镜头，保持总帧数不变",
		s.ID, s.DurationSeconds, canvas.FPS, frames, lower/fps, int(lower), upper/fps, int(upper))}}
}
