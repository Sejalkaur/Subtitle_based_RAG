// ms <-> hh:mm:ss.mmm utilities

export function msToTimestamp(ms) {
  const sign = ms < 0 ? '-' : '';
  ms = Math.abs(ms);

  const hours = Math.floor(ms / 3_600_000);
  ms %= 3_600_000;
  const minutes = Math.floor(ms / 60_000);
  ms %= 60_000;
  const seconds = Math.floor(ms / 1000);
  const millis = ms % 1000;

  const hh = String(hours).padStart(2, '0');
  const mm = String(minutes).padStart(2, '0');
  const ss = String(seconds).padStart(2, '0');
  const mmm = String(millis).padStart(3, '0');
  return `${sign}${hh}:${mm}:${ss}.${mmm}`;
}

export function secondsToMs(sec) {
  return Math.round(Number(sec) * 1000);
}

export function msToSeconds(ms) {
  return ms / 1000;
}
