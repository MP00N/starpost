/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
/**
 * Lens filter for the liquid-glass panel.
 *
 * Apple's Liquid Glass is defined by *displacement*, not blur: the material
 * compresses and bends what sits behind it near the rim. `backdrop-filter` can
 * only blur, so there is no way to reach that look with CSS alone.
 *
 * The workaround used here: the panel repaints the hero's decorative layer (the
 * colour blobs and the grid — both ours, both cheap) inside itself, and runs an
 * SVG displacement map over that copy. Real lensing of real content, without
 * WebGL and without a shader library.
 *
 * Two knobs matter:
 *   baseFrequency — lower is a wider, softer bend, like thick glass; higher is
 *                   a bubbly distortion that reads as cheap plastic.
 *   scale         — peak displacement in pixels. Above ~24 the grid tears.
 *
 * The map is static. Animating feDisplacementMap forces a full-surface repaint
 * every frame and is the fastest way to drop a phone to 20fps.
 */
export function LensFilter() {
  return (
    <svg
      aria-hidden='true'
      width='0'
      height='0'
      style={{ position: 'absolute', pointerEvents: 'none' }}
    >
      <defs>
        <filter
          id='sp-lens'
          x='-20%'
          y='-20%'
          width='140%'
          height='140%'
          colorInterpolationFilters='sRGB'
        >
          <feTurbulence
            type='fractalNoise'
            baseFrequency='0.008 0.014'
            numOctaves='2'
            seed='5'
            result='bend'
          />
          <feDisplacementMap
            in='SourceGraphic'
            in2='bend'
            scale='18'
            xChannelSelector='R'
            yChannelSelector='G'
            result='bent'
          />
          <feGaussianBlur in='bent' stdDeviation='6' />
        </filter>

        {/* A narrower band for the rim: thicker glass bends harder at the edge. */}
        <filter
          id='sp-lens-edge'
          x='-20%'
          y='-20%'
          width='140%'
          height='140%'
          colorInterpolationFilters='sRGB'
        >
          <feTurbulence
            type='fractalNoise'
            baseFrequency='0.02 0.03'
            numOctaves='2'
            seed='9'
            result='bend2'
          />
          <feDisplacementMap
            in='SourceGraphic'
            in2='bend2'
            scale='30'
            xChannelSelector='R'
            yChannelSelector='G'
            result='bent2'
          />
          <feGaussianBlur in='bent2' stdDeviation='2.5' />
        </filter>
      </defs>
    </svg>
  )
}
