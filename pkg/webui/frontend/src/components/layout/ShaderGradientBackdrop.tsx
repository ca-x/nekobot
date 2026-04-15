import { useEffect, useMemo, useState } from 'react';
import { ShaderGradient, ShaderGradientCanvas } from '@shader-gradient/react';

type ShaderBackdropVariant = 'login' | 'init' | 'app';

interface ShaderGradientBackdropProps {
  variant: ShaderBackdropVariant;
  className?: string;
}

function useDarkMode() {
  const [darkMode, setDarkMode] = useState(() =>
    document.documentElement.classList.contains('dark'),
  );

  useEffect(() => {
    const root = document.documentElement;
    const observer = new MutationObserver(() => {
      setDarkMode(root.classList.contains('dark'));
    });

    observer.observe(root, { attributes: true, attributeFilter: ['class'] });
    return () => observer.disconnect();
  }, []);

  return darkMode;
}

function useReducedMotion() {
  const [reducedMotion, setReducedMotion] = useState(false);

  useEffect(() => {
    const media = window.matchMedia('(prefers-reduced-motion: reduce)');
    const sync = () => setReducedMotion(media.matches);

    sync();
    media.addEventListener('change', sync);
    return () => media.removeEventListener('change', sync);
  }, []);

  return reducedMotion;
}

export default function ShaderGradientBackdrop({
  variant,
  className,
}: ShaderGradientBackdropProps) {
  const darkMode = useDarkMode();
  const reducedMotion = useReducedMotion();

  const gradientProps = useMemo(() => {
    if (variant === 'init') {
      return darkMode
        ? {
            preset: 'interstella' as const,
            type: 'sphere' as const,
            shader: 'cosmic' as const,
            lightType: 'env' as const,
            envPreset: 'lobby' as const,
            grain: true,
            grainBlending: 0.12,
            brightness: 0.7,
            cDistance: 0.8,
            cameraZoom: 14,
          }
        : {
            preset: 'mint' as const,
            type: 'waterPlane' as const,
            shader: 'glass' as const,
            lightType: 'env' as const,
            envPreset: 'dawn' as const,
            grain: false,
            brightness: 1.05,
            cDistance: 4.2,
            cameraZoom: 1,
          };
    }

    if (variant === 'app') {
      return darkMode
        ? {
            preset: 'nightyNight' as const,
            type: 'waterPlane' as const,
            shader: 'cosmic' as const,
            lightType: 'env' as const,
            envPreset: 'lobby' as const,
            grain: false,
            brightness: 0.48,
            cDistance: 3.4,
            cameraZoom: 6.5,
          }
        : {
            preset: 'halo' as const,
            type: 'plane' as const,
            shader: 'glass' as const,
            lightType: 'env' as const,
            envPreset: 'dawn' as const,
            grain: false,
            brightness: 0.88,
            cDistance: 4.6,
            cameraZoom: 0.95,
          };
    }

    return darkMode
      ? {
          preset: 'pensive' as const,
          type: 'sphere' as const,
          shader: 'cosmic' as const,
          lightType: 'env' as const,
          envPreset: 'lobby' as const,
          grain: true,
          grainBlending: 0.16,
          brightness: 0.8,
          cDistance: 1.3,
          cameraZoom: 11.5,
        }
      : {
          preset: 'halo' as const,
          type: 'plane' as const,
          shader: 'glass' as const,
          lightType: 'env' as const,
          envPreset: 'dawn' as const,
          grain: true,
          grainBlending: 0.08,
          brightness: 1.05,
          cDistance: 3.8,
          cameraZoom: 1.1,
        };
  }, [darkMode, variant]);

  if (reducedMotion) {
    return null;
  }

  return (
    <ShaderGradientCanvas
      className={className}
      style={{ width: '100%', height: '100%' }}
      pixelDensity={variant === 'app' ? 1 : 1.2}
      fov={45}
      pointerEvents="none"
      lazyLoad
      threshold={0.05}
      powerPreference="high-performance"
    >
      <ShaderGradient {...gradientProps} />
    </ShaderGradientCanvas>
  );
}
