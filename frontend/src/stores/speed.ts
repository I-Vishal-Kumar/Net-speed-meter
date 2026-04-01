import { writable } from 'svelte/store'
import { EventsOn } from '../../wailsjs/runtime/runtime'

export type SpeedData = {
  Download: number
  Upload: number
  Iface: string
  PeakDown: number
  PeakUp: number
  SessionDown: number
  SessionUp: number
  TodayDown: number
  TodayUp: number
}

export const speed = writable<SpeedData>({
  Download: 0,
  Upload: 0,
  Iface: '',
  PeakDown: 0,
  PeakUp: 0,
  SessionDown: 0,
  SessionUp: 0,
  TodayDown: 0,
  TodayUp: 0,
})

EventsOn('speed', (data: SpeedData) => {
  speed.set(data)
})
