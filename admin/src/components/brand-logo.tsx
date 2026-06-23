import type { HTMLAttributes, SVGProps } from "react";

import { cn } from "@/lib/utils";

const buzzHiveBackground = "radial-gradient(circle at 28% 20%, rgba(255,255,255,.34), transparent 58%), linear-gradient(132deg, #7c3aed 0%, #4f46e5 52%, #2563eb 100%)";

export function BrandLogo(props: SVGProps<SVGSVGElement>) {
  return (
    <svg viewBox="6 6 52 52" fill="none" aria-hidden="true" {...props}>
      <path
        d="M32 8.48 52.33 20.24v23.52L32 55.52 11.67 43.76V20.24L32 8.48Z"
        stroke="currentColor"
        strokeWidth="2.85"
        strokeLinejoin="round"
      />
      <path
        d="M26.54 31.16c-4.28-4.03-9.91-2.69-11.09 2.02-1.01 4.2 4.03 6.8 10.25 4.54M37.46 31.16c4.28-4.03 9.91-2.69 11.09 2.02 1.01 4.2-4.03 6.8-10.25 4.54"
        stroke="currentColor"
        strokeWidth="2.18"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <path
        d="M26.54 30.32a5.46 5.46 0 0 1 10.92 0v9.24a5.46 5.46 0 0 1-10.92 0V30.32Z"
        stroke="currentColor"
        strokeWidth="2.85"
        strokeLinejoin="round"
      />
      <path d="M26.54 33.85h10.92v4.45H26.54Z" fill="currentColor" />
      <path d="M26.96 33.85h10.08M26.96 38.3h10.08" stroke="currentColor" strokeWidth="2.52" strokeLinecap="round" />
      <path
        d="M29.48 24.44c-1.01-3.36-3.02-5.04-5.29-4.45-1.68.42-2.35 1.76-2.27 3.61M34.52 24.44c1.01-3.36 3.02-5.04 5.29-4.45 1.68.42 2.35 1.76 2.27 3.61"
        stroke="currentColor"
        strokeWidth="2.18"
        strokeLinecap="round"
      />
    </svg>
  );
}

type BrandIconProps = HTMLAttributes<HTMLSpanElement> & {
  iconClassName?: string;
  shape?: "rounded" | "circle";
};

export function BrandIcon({
  className,
  iconClassName,
  shape = "rounded",
  style,
  ...props
}: BrandIconProps) {
  return (
    <span
      className={cn(
        "inline-grid shrink-0 place-items-center overflow-hidden text-white",
        shape === "circle" ? "rounded-full" : "rounded-lg",
        className,
      )}
      style={{ backgroundImage: buzzHiveBackground, ...style }}
      {...props}
    >
      <BrandLogo className={cn("block size-[60%]", iconClassName)} />
    </span>
  );
}
