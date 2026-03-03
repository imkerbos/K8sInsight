import dayjs from 'dayjs'
import utc from 'dayjs/plugin/utc'
import timezone from 'dayjs/plugin/timezone'

dayjs.extend(utc)
dayjs.extend(timezone)

const DEFAULT_TZ = 'Asia/Shanghai'

// 默认使用北京时间（UTC+8）
dayjs.tz.setDefault(DEFAULT_TZ)

const wrapped = ((...args: Parameters<typeof dayjs>) => {
  const d = args.length > 0 ? dayjs(...args) : dayjs()
  return d.tz(DEFAULT_TZ)
}) as typeof dayjs

Object.assign(wrapped, dayjs)
wrapped.unix = ((ts: number) => dayjs.unix(ts).tz(DEFAULT_TZ)) as typeof dayjs.unix

export default wrapped
