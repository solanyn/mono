import { QuartzComponent, QuartzComponentConstructor, QuartzComponentProps } from "./types"
// @ts-ignore
import script from "./scripts/nowplaying.inline"

export default (() => {
  const NowPlaying: QuartzComponent = ({ displayClass, cfg }: QuartzComponentProps) => {
    return <div id="nowplaying" />
  }

  NowPlaying.afterDOMLoaded = script

  return NowPlaying
}) satisfies QuartzComponentConstructor
