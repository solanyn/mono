declare module 'recharts' {
  import type { FC, SVGProps } from 'react'

  export interface LineChartProps {
    data?: unknown[]
    children?: React.ReactNode
    width?: number
    height?: number
    margin?: { top?: number; right?: number; bottom?: number; left?: number }
  }
  export const LineChart: FC<LineChartProps>

  export interface LineProps {
    type?: string
    dataKey: string
    stroke?: string
    dot?: boolean
    strokeWidth?: number
  }
  export const Line: FC<LineProps>

  export interface XAxisProps {
    dataKey?: string
    stroke?: string
    tick?: object
  }
  export const XAxis: FC<XAxisProps>

  export interface YAxisProps {
    stroke?: string
    tick?: object
    domain?: [number, number]
  }
  export const YAxis: FC<YAxisProps>

  export interface CartesianGridProps {
    strokeDasharray?: string
    stroke?: string
  }
  export const CartesianGrid: FC<CartesianGridProps>

  export interface TooltipProps {
    contentStyle?: object
  }
  export const Tooltip: FC<TooltipProps>

  export interface ResponsiveContainerProps {
    width?: string | number
    height?: string | number
    children?: React.ReactNode
  }
  export const ResponsiveContainer: FC<ResponsiveContainerProps>
}
