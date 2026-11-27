/**
 * Android 键盘状态判定。
 *
 * app 的 manifest 用 `windowSoftInputMode="adjustResize"`：键盘弹出时窗口自身被压缩，
 * `window.innerHeight` 与 `visualViewport.height` 一起变小，两者差值恒为 0。
 * 因此「键盘是否打开」不能看两者之差，只能和「无键盘时的视口高度」基线比较。
 *
 * 基线只在没有文本控件聚焦时刷新：这样键盘弹出造成的收缩不会被当成新的正常高度，
 * 而旋转、分屏这类没有键盘的视口变化仍然能立刻更新基线。
 */

export interface AndroidViewportSample {
  /** window.innerHeight */
  innerHeight: number;
  /** visualViewport.height，不可用时传 0 */
  visualHeight: number;
  /** visualViewport.offsetTop，不可用时传 0 */
  visualTop: number;
  /** 当前焦点是否落在文本输入控件上（此时键盘可能弹出，基线不能被压低） */
  editing: boolean;
}

export interface AndroidViewportState {
  /** 窗口没有随键盘收缩（adjustPan）时，被键盘挡掉的高度；adjustResize 下恒为 0。 */
  keyboardInset: number;
  keyboardOpen: boolean;
  /** 下一次计算要传回的基线高度 */
  baselineHeight: number;
}

/** 判定阈值：小于这个高度差视为浏览器控件抖动，不算键盘。 */
export const ANDROID_KEYBOARD_DETECT_THRESHOLD_PX = 48;

export function computeAndroidViewportState(previousBaseline: number, sample: AndroidViewportSample): AndroidViewportState {
  const visualHeight = Number.isFinite(sample.visualHeight) && sample.visualHeight > 0 ? sample.visualHeight : sample.innerHeight;
  const visualTop = Number.isFinite(sample.visualTop) && sample.visualTop > 0 ? sample.visualTop : 0;
  const viewportHeight = visualHeight > 0 ? visualHeight : sample.innerHeight;

  // 首次采样若正好在输入中，手里没有「无键盘高度」，只能先把它当基线（这一次不判键盘）。
  const hasBaseline = previousBaseline > 0;
  const baselineHeight = sample.editing && hasBaseline ? Math.max(previousBaseline, viewportHeight) : viewportHeight;
  const occluded = Math.round(baselineHeight - viewportHeight - visualTop);
  // 只有窗口没被键盘压缩时才有额外的遮挡高度，滚动留白要用它。
  const keyboardInset = Math.max(0, Math.round(sample.innerHeight - viewportHeight - visualTop));

  return {
    baselineHeight,
    keyboardInset,
    keyboardOpen: occluded > ANDROID_KEYBOARD_DETECT_THRESHOLD_PX,
  };
}
