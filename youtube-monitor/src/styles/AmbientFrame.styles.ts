import { css, keyframes } from '@emotion/react'
import { Constants } from '../lib/constants'

export const themedRoot = css`
	--ambient-shell-bg: rgba(255, 255, 255, 0.2);
	--ambient-panel-bg: rgba(255, 255, 255, 0.3);
	--ambient-panel-strong-bg: rgba(255, 255, 255, 0.4);
	--ambient-panel-dark-bg: rgba(53, 49, 49, 0.3);
	--ambient-border: rgba(255, 255, 255, 0.34);
	--ambient-highlight: rgba(221, 229, 255, 0.42);
	--ambient-text-primary: ${Constants.primaryTextColor};
	--ambient-text-secondary: ${Constants.secondaryTextColor};
	--ambient-text-muted: #4b5563;
	--ambient-command-bg: rgba(0, 0, 0, 0.08);
	--ambient-ticker-chip-bg: rgba(255, 255, 255, 0.7);
	--ambient-notice: #4763d7;
	--ambient-animation-opacity: 0.1;

	&[data-time-theme='dawn'] {
		--ambient-shell-bg: rgba(216, 229, 251, 0.28);
		--ambient-panel-bg: rgba(232, 240, 255, 0.4);
		--ambient-panel-strong-bg: rgba(242, 247, 255, 0.5);
		--ambient-panel-dark-bg: rgba(48, 54, 82, 0.46);
		--ambient-border: rgba(218, 231, 255, 0.58);
		--ambient-highlight: rgba(188, 202, 255, 0.48);
		--ambient-text-primary: #34266f;
		--ambient-text-secondary: #f3f5ff;
		--ambient-text-muted: #4a5574;
		--ambient-command-bg: rgba(58, 70, 110, 0.1);
		--ambient-ticker-chip-bg: rgba(245, 248, 255, 0.76);
		--ambient-notice: #425dbd;
		--ambient-animation-opacity: 0.13;
	}

	&[data-time-theme='sunset'] {
		--ambient-shell-bg: rgba(255, 229, 214, 0.28);
		--ambient-panel-bg: rgba(255, 238, 227, 0.4);
		--ambient-panel-strong-bg: rgba(255, 246, 238, 0.5);
		--ambient-panel-dark-bg: rgba(73, 53, 57, 0.44);
		--ambient-border: rgba(255, 217, 194, 0.56);
		--ambient-highlight: rgba(255, 194, 158, 0.46);
		--ambient-text-primary: #4a285f;
		--ambient-text-secondary: #fff4ef;
		--ambient-text-muted: #62505a;
		--ambient-command-bg: rgba(116, 62, 46, 0.1);
		--ambient-ticker-chip-bg: rgba(255, 249, 243, 0.76);
		--ambient-notice: #7553a2;
		--ambient-animation-opacity: 0.13;
	}

	&[data-time-theme='night'] {
		--ambient-shell-bg: rgba(37, 43, 82, 0.6);
		--ambient-panel-bg: rgba(56, 64, 110, 0.72);
		--ambient-panel-strong-bg: rgba(69, 78, 130, 0.76);
		--ambient-panel-dark-bg: rgba(26, 28, 55, 0.78);
		--ambient-border: rgba(181, 194, 255, 0.28);
		--ambient-highlight: rgba(150, 171, 255, 0.38);
		--ambient-text-primary: #f1efff;
		--ambient-text-secondary: #f5f2ff;
		--ambient-text-muted: #d9dcf2;
		--ambient-command-bg: rgba(12, 14, 34, 0.28);
		--ambient-ticker-chip-bg: rgba(86, 96, 147, 0.84);
		--ambient-notice: #d5dcff;
		--ambient-animation-opacity: 0.1;
	}

	&[data-time-theme='midnight'] {
		--ambient-shell-bg: rgba(29, 33, 59, 0.66);
		--ambient-panel-bg: rgba(46, 50, 77, 0.78);
		--ambient-panel-strong-bg: rgba(56, 61, 90, 0.82);
		--ambient-panel-dark-bg: rgba(22, 24, 42, 0.84);
		--ambient-border: rgba(166, 176, 214, 0.24);
		--ambient-highlight: rgba(135, 145, 190, 0.32);
		--ambient-text-primary: #eeeefa;
		--ambient-text-secondary: #f1f0f8;
		--ambient-text-muted: #d0d3e1;
		--ambient-command-bg: rgba(9, 11, 25, 0.3);
		--ambient-ticker-chip-bg: rgba(68, 73, 103, 0.88);
		--ambient-notice: #cbd1ea;
		--ambient-animation-opacity: 0.075;
	}
`

const driftRight = keyframes`
	0% {
		background-position: 15% 0%, 85% 70%;
		opacity: calc(var(--ambient-animation-opacity) * 0.7);
	}
	100% {
		background-position: 65% 35%, 35% 100%;
		opacity: var(--ambient-animation-opacity);
	}
`

const driftBottom = keyframes`
	0% {
		background-position: 0% 50%, 70% 50%;
		opacity: calc(var(--ambient-animation-opacity) * 0.65);
	}
	100% {
		background-position: 70% 50%, 20% 50%;
		opacity: calc(var(--ambient-animation-opacity) * 0.9);
	}
`

const lightLayer = css`
	position: absolute;
	pointer-events: none;
	z-index: 30;
	overflow: hidden;
	color: var(--ambient-highlight);
	background-repeat: no-repeat;
	transition: color 30s ease, opacity 30s ease;

	@media (prefers-reduced-motion: reduce) {
		animation: none;
		opacity: calc(var(--ambient-animation-opacity) * 0.75);
	}
`

export const rightLight = css`
	${lightLayer};
	top: 0;
	right: 0;
	width: ${Constants.sideBarWidth}px;
	height: ${Constants.screenHeight}px;
	background-image:
		radial-gradient(circle at center, currentColor 0%, transparent 68%),
		linear-gradient(125deg, transparent 20%, currentColor 50%, transparent 80%);
	background-size: 135% 45%, 180% 30%;
	animation: ${driftRight} 78s ease-in-out infinite alternate;
`

export const bottomLight = css`
	${lightLayer};
	bottom: 0;
	left: 0;
	width: ${Constants.screenWidth - Constants.sideBarWidth}px;
	height: ${Constants.messageBarHeight}px;
	background-image:
		radial-gradient(ellipse at center, currentColor 0%, transparent 72%),
		linear-gradient(100deg, transparent 15%, currentColor 50%, transparent 85%);
	background-size: 45% 160%, 55% 100%;
	animation: ${driftBottom} 86s ease-in-out infinite alternate;
`
