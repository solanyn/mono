declare module 'recharts' {
  import type { FC } from 'react'

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
    dot?: boolean | object
    strokeWidth?: number
    strokeDasharray?: string
    opacity?: number
  }
  export const Line: FC<LineProps>

  export interface XAxisProps {
    dataKey?: string
    stroke?: string
    tick?: object
    type?: string
    allowDuplicatedCategory?: boolean
    data?: unknown[]
  }
  export const XAxis: FC<XAxisProps>

  export interface YAxisProps {
    stroke?: string
    tick?: object
    domain?: [string | number, string | number]
    dataKey?: string
    tickFormatter?: (value: number) => string
    reversed?: boolean
  }
  export const YAxis: FC<YAxisProps>

  export interface CartesianGridProps {
    strokeDasharray?: string
    stroke?: string
  }
  export const CartesianGrid: FC<CartesianGridProps>

  export interface TooltipProps {
    contentStyle?: object
    formatter?: (...args: unknown[]) => unknown
    labelFormatter?: (label: string) => string
  }
  export const Tooltip: FC<TooltipProps>

  export interface ResponsiveContainerProps {
    width?: string | number
    height?: string | number
    children?: React.ReactNode
  }
  export const ResponsiveContainer: FC<ResponsiveContainerProps>

  export interface ScatterChartProps {
    data?: unknown[]
    children?: React.ReactNode
    width?: number
    height?: number
    margin?: { top?: number; right?: number; bottom?: number; left?: number }
  }
  export const ScatterChart: FC<ScatterChartProps>

  export interface ScatterProps {
    data?: unknown[]
    children?: React.ReactNode
    fill?: string
    dataKey?: string
  }
  export const Scatter: FC<ScatterProps>

  export interface CellProps {
    fill?: string
    opacity?: number
  }
  export const Cell: FC<CellProps>
}
