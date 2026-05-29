import type { SVGProps } from "react";

export function BrandLogo(props: SVGProps<SVGSVGElement>) {
  return (
    <svg viewBox="0 0 64 64" fill="none" aria-hidden="true" {...props}>
      <path
        d="M32 4 56.2 18v28L32 60 7.8 46V18L32 4Z"
        stroke="currentColor"
        strokeWidth="3.4"
        strokeLinejoin="round"
      />
      <g>
        <path
          d="M25.5 31c-5.1-4.8-11.8-3.2-13.2 2.4-1.2 5 4.8 8.1 12.2 5.4M38.5 31c5.1-4.8 11.8-3.2 13.2 2.4 1.2 5-4.8 8.1-12.2 5.4"
          stroke="currentColor"
          strokeWidth="2.6"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
      </g>
      <g>
        <path
          d="M25.5 30a6.5 6.5 0 0 1 13 0v11a6.5 6.5 0 0 1-13 0V30Z"
          stroke="currentColor"
          strokeWidth="3.4"
          strokeLinejoin="round"
        />
        <path d="M25.5 34.2h13v5.3h-13Z" fill="currentColor" />
        <path d="M26 34.2h12M26 39.5h12" stroke="currentColor" strokeWidth="3" strokeLinecap="round" />
        <path
          d="M29 23c-1.2-4-3.6-6-6.3-5.3-2 .5-2.8 2.1-2.7 4.3M35 23c1.2-4 3.6-6 6.3-5.3 2 .5 2.8 2.1 2.7 4.3"
          stroke="currentColor"
          strokeWidth="2.6"
          strokeLinecap="round"
        />
      </g>
    </svg>
  );
}
