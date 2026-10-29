#!/usr/bin/env bash

# OAuthは最短7手の外部発行後の拒否に加え、途中の制限・解除を含めて検証する。
# 他の既存仕様の深度は維持し、gateとHTMLで同じ既定値を使う。
fsl_default_depth() {
	case "$1" in
		mypage_oauth) printf '12' ;;
		*) printf '8' ;;
	esac
}
