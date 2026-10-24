import { Link } from '@tanstack/react-router'
import type { ReactNode } from 'react'
import { ExternalLink } from './features/mypage/view'
import {
	type PublicPolicyConfig,
	readPublicPolicyConfig,
	validatePublicPolicyConfig,
} from './public-policy-config'

// Public drafts v1 (2026-10-05), with 05 Privacy/09 decision clarifications:
// https://app.notion.com/p/3ef357a8d0ce8175aeaed4c044111cfe
// https://app.notion.com/p/3ef357a8d0ce8196a4b1f71482c1015a
// https://app.notion.com/p/3ed357a8d0ce81e7b7d2cc85ec8ab4cf (05)
// https://app.notion.com/p/3f0357a8d0ce81a5b0e4e108231bda0f (09)
// Operator identity remains available on request under 05; an optional public
// service/operator label does not require disclosure of a private real name.
// Injecting D03 values does not approve policies or close D01/D02/operational gates.
export type PublicPolicyProps = { publicPolicyInputs?: unknown }

function policyConfig(inputs: unknown): PublicPolicyConfig {
	return (
		inputs === undefined
			? readPublicPolicyConfig()
			: validatePublicPolicyConfig(inputs)
	).config
}

function DraftNotice() {
	return (
		<>
			<p className="notice" role="status">
				公開前の確認用文面です。運営主体・問い合わせ先・施行日・公開条件は、確定後に掲載します。
			</p>
			<p>
				公開ドラフト v1（2026-10-05）です。実装・運用・Google
				OAuth審査・最新法令との最終照合前であり、公開承認済みの文面ではありません。
			</p>
		</>
	)
}

function ContactLink({
	href,
	children,
}: {
	href: string
	children: ReactNode
}) {
	return href.startsWith('mailto:') ? (
		<a className="external-link" href={href} rel="noreferrer">
			{children}
		</a>
	) : (
		<ExternalLink href={href}>{children}</ExternalLink>
	)
}

export function PublicPolicyContact({
	publicPolicyInputs,
}: PublicPolicyProps = {}) {
	const config = policyConfig(publicPolicyInputs)
	return (
		<>
			{config.operatorName && (
				<p>運営者・サービスの公開表記：{config.operatorName}</p>
			)}
			{config.contactURL && (
				<p>
					<ContactLink href={config.contactURL}>一般のお問い合わせ</ContactLink>
				</p>
			)}
			{config.privacyContactURL ? (
				<p>
					<ContactLink href={config.privacyContactURL}>
						個人情報・プライバシーに関する問い合わせ・請求
					</ContactLink>
				</p>
			) : (
				<p>
					個人情報・プライバシーに関する返信可能な専用窓口は、公開前に確定して掲載します。
				</p>
			)}
			<p>
				運営者の氏名または名称・住所その他、法令上本人の知り得る状態に置く必要がある情報は、窓口への請求に応じ、法令に従い遅滞なく回答する方針です。
			</p>
		</>
	)
}

function EffectiveDate({ value }: { value?: string }) {
	return (
		<p>
			施行日：
			{value ? <time dateTime={value}>{value}</time> : '[要確定：施行日]'}
		</p>
	)
}

export function PrivacyPolicy({ publicPolicyInputs }: PublicPolicyProps = {}) {
	const config = policyConfig(publicPolicyInputs)
	return (
		<main className="main-content public-page">
			<article className="card policy-page">
				<h1>プライバシーポリシー</h1>
				<DraftNotice />
				<h2>1. はじめに</h2>
				<p>
					Online Study
					Space（以下「本サービス」といいます。）は、YouTube上のオンライン作業空間とWeb上のマイページを組み合わせ、利用者の作業状態や作業履歴を表示するサービスです。
				</p>
				<p>
					本プライバシーポリシーは、本サービスが取得・利用・保存する情報、その利用目的、外部サービスとの関係、利用者が情報の削除等を求める方法について定めるものです。
				</p>
				<p>
					運営者の氏名または名称・住所等は、窓口への請求に応じて遅滞なく回答する方針です。
				</p>
				<h2>2. 取得・利用する情報</h2>
				<p>
					本サービスでは、サービス提供に必要な範囲で、次の情報を取得または生成します。
				</p>
				<h3>2.1 YouTubeアカウント・チャンネルに関する情報</h3>
				<ul>
					<li>YouTube channel ID</li>
					<li>YouTube channelの表示名</li>
					<li>YouTube handle</li>
					<li>YouTube channelのプロフィール画像</li>
					<li>YouTube OAuthによる本人確認に必要な一時的な認証情報</li>
				</ul>
				<p>
					本サービスでは、YouTube channel IDをOnline Study
					Spaceのアカウント識別子として利用します。
				</p>
				<h3>2.2 Online Study Spaceの利用情報</h3>
				<ul>
					<li>入室・退室の状態</li>
					<li>作業中・休憩中等の状態</li>
					<li>作業名</li>
					<li>作業開始・終了等の時刻</li>
					<li>作業時間</li>
					<li>Room・Seat等、本サービス上の利用状態</li>
					<li>累計作業時間その他、本サービスが独自に管理する状態</li>
				</ul>
				<p>
					作業名等は、利用者がYouTubeライブチャット上で本サービス向けのコマンドとして入力した内容から取得する場合があります。
				</p>
				<p>
					本サービスは、YouTubeライブチャット本文そのものを作業履歴として無期限に保存することを目的としていません。
				</p>
				<h3>2.3 Web利用・認証に関する情報</h3>
				<ul>
					<li>
						問い合わせに関する受付情報と、依頼目的に対応する本人確認の一時的な情報
					</li>
					<li>Firebase Authenticationのユーザー識別情報</li>
					<li>初回Webログイン時刻</li>
					<li>本プライバシーポリシーへの同意version・同意時刻</li>
					<li>
						セキュリティおよび障害調査に必要なrequest ID、HTTP
						status、latencyその他の技術情報
					</li>
				</ul>
				<p>
					本サービスは、Firebase ID token、Firebase refresh token、Firebase
					Custom Token、YouTube OAuth authorization code、YouTube OAuth access
					token等のcredential本文を、通常のアプリケーションログへ記録しません。
				</p>
				<h3>2.4 Analytics</h3>
				<p>
					利用者が明示的に許可した場合に限り、Google Analytics
					4（GA4）を利用し、ページ閲覧や本サービス上の操作等の情報を取得する場合があります。
				</p>
				<p>
					GA4には、YouTube channel ID、Firebase uid、メールアドレス、YouTube
					channelの表示名、作業名等の直接的なサービスユーザー識別情報を送信しません。
				</p>
				<h2>3. 利用目的</h2>
				<p>取得した情報は、次の目的で利用します。</p>
				<ol>
					<li>
						本人が利用するYouTube
						channelを確認し、本サービスへログインできるようにするため
					</li>
					<li>
						YouTube channel IDに対応するOnline Study
						Spaceの作業履歴・現在状態・統計等を表示するため
					</li>
					<li>本サービスの提供、維持、障害対応および不正利用防止のため</li>
					<li>
						利用者からの問い合わせ、データ削除、開示等の請求に対応するため
					</li>
					<li>
						利用者がAnalyticsを許可した場合に、本サービスの導線・UI・利用状況を分析し、改善するため
					</li>
					<li>法令または利用規約に基づく対応を行うため</li>
				</ol>
				<p>
					取得した情報を、本サービスの主要機能提供・改善と無関係な広告ターゲティングのために販売または転用しません。
				</p>
				<h2>4. YouTube API Servicesの利用</h2>
				<p>本サービスは、YouTube API Servicesを利用します。</p>
				<p>
					ログイン時には、YouTube Data
					APIのread-only権限を利用し、OAuth認証を行ったGoogleアカウントについて、YouTube
					APIが対象としているYouTube channelを確認します。
				</p>
				<p>
					本サービスは、Phase 0A〜1において、ログイン時の本人確認のためにYouTube
					APIを利用し、YouTubeへの動画投稿、コメント投稿、編集または削除を行いません。
				</p>
				<p>
					YouTube API Servicesの利用に関しては、YouTube API Services Terms of
					ServiceおよびYouTube API Services Developer Policiesが適用されます。
				</p>
				<ul>
					<li>
						<ExternalLink href="https://www.youtube.com/t/terms">
							YouTube Terms of Service
						</ExternalLink>
					</li>
					<li>
						<ExternalLink href="https://developers.google.com/youtube/terms/api-services-terms-of-service">
							YouTube API Services Terms of Service
						</ExternalLink>
					</li>
					<li>
						<ExternalLink href="https://policies.google.com/privacy">
							Google Privacy Policy
						</ExternalLink>
					</li>
				</ul>
				<h2>5. Google OAuth credentialの取扱い</h2>
				<p>
					本サービスは、ログイン時にGoogle OAuth Authorization Code
					Flowを利用します。
				</p>
				<p>
					Phase 0A〜1では、YouTube OAuth refresh
					tokenを要求・保存しません。ログイン時に取得した短命のYouTube OAuth
					access tokenは、対象YouTube
					channelの確認に使用した後、継続的なサービスsessionとして保存しません。
				</p>
				<p>
					YouTube channelの確認後は、確認済みのYouTube channel IDをFirebase
					Authenticationのuidとして使用し、Firebase
					Authenticationのsessionによってログイン状態を維持します。
				</p>
				<p>
					そのため、Google側でOAuth権限を取り消した場合でも、既存のFirebase
					sessionが直ちに終了しない場合があります。
				</p>
				<h2>6. YouTube channel情報の保存・更新</h2>
				<p>
					YouTube
					channelの表示名、handle、プロフィール画像等の可変な公開metadataは、サービス表示のため一時的にcacheする場合があります。
				</p>
				<p>
					これらのmetadataは定期的に再取得し、30暦日以内に更新または削除する方針です。最新情報を取得できない場合、古い情報を現在の情報として表示し続けず、名前・画像等の表示を一時的に省略することがあります。表示を止めるだけで保存を続けず、未利用のチャンネルも期限内の削除対象とします。
				</p>
				<p>
					YouTube channel IDは、本サービスのアカウント識別子として利用します。
				</p>
				<h2>7. 作業履歴の取扱い</h2>
				<p>
					本サービスは、利用者がOnline Study
					Spaceで行った作業に関する情報を、本サービス固有の作業履歴として保存します。
				</p>
				<p>
					これには、作業名、work /
					break等の状態、作業segment、作業時間、Room・Seat等が含まれる場合があります。
				</p>
				<p>
					作業名に個人情報、機密情報その他第三者へ知られることが望ましくない情報を入力した場合、その内容が本サービスの作業履歴として保存される場合があります。利用者は、作業名へ入力する内容に注意してください。
				</p>
				<h2>8. Analytics・Cookie等</h2>
				<p>
					認証・セキュリティのためFirebase Authentication、Firebase App Check /
					reCAPTCHA Enterprise等を利用します。これらの必須通信は任意のGA4
					Analyticsとは別で、AnalyticsをOFFにしてもサービス利用に必要な通信は行われます。送信情報・送信先の詳細は公開前に実際の構成と照合します。
				</p>
				<p>本サービスのAnalyticsはdefaultでOFFです。</p>
				<p>
					利用者が明示的に「Analyticsを許可」した場合に限り、GA4を初期化し、その後の利用状況を送信します。許可前に発生したイベントを、後から遡ってGA4へ送信しません。
				</p>
				<p>
					Analyticsを許可しなくても、YouTubeログインやマイページ等の主要機能を利用できます。
				</p>
				<p>
					Analyticsの選択状態は、browser単位のfirst-party
					storageへ保存する場合があります。この選択状態をYouTube channel
					IDやFirebase uidと結び付けません。
				</p>
				<p>
					利用者は、Cookie設定からAnalyticsをいつでもOFFにできます。OFFにした後は、将来のGA4送信を停止します。
				</p>
				<p>
					計測の公開案では、安全な画面の種類、ログイン等の操作、固定の流入元だけをGoogleへ送り、本サービスの導線・UI・利用状況の分析に使います。任意のquery、OAuthのcode・state・error、依頼の本人確認情報、本人別のAPI応答は送りません。広告配信、広告パーソナライズ、Google
					Signals、User-IDは使いません。
				</p>
				<p>
					GA4の送信情報、送信先での利用目的、保持設定、停止方法は、公開前に実際の通信・Google側の設定と最終照合します。
				</p>
				<h2>9. 利用する外部サービス</h2>
				<p>
					本サービスでは、機能提供・認証・hosting・データ保存・分析等のため、次の外部サービスを利用する場合があります。
				</p>
				<ul>
					<li>Google / YouTube API Services</li>
					<li>Firebase Authentication</li>
					<li>Google Cloud Platform</li>
					<li>Google Analytics 4</li>
					<li>その他、本サービスの運営に必要なインフラストラクチャ</li>
				</ul>
				<p>
					これらのサービスでは、各提供者の規約・プライバシーポリシーに従って情報が処理されます。サービス提供者のインフラストラクチャ上で、情報が日本国外を含む地域で取り扱われる場合があります。
				</p>
				<h2>10. 保存期間</h2>
				<p>情報は、それぞれの利用目的に必要な期間に限って保持します。</p>
				<ul>
					<li>
						YouTube OAuth authorization code / access
						token：ログイン処理に必要な短期間
					</li>
					<li>
						OAuth login transaction：原則10分で利用不能とし、その後cleanup
					</li>
					<li>
						YouTube
						channelの可変metadata：通常24時間程度を更新目安とし、30日以上古い情報へ依存しない
					</li>
					<li>
						Online Study
						Spaceの作業履歴・アカウント情報：サービス提供に必要な期間
					</li>
					<li>Analyticsデータ：GA4の設定およびGoogleの仕様に従う</li>
					<li>運用ログ：セキュリティ・障害調査に必要な期間</li>
				</ul>
				<p>
					法令上保存が必要な場合は、その必要な範囲で保存することがあります。
				</p>
				<h2>11. Google OAuth権限の取消し</h2>
				<p>
					利用者は、Google Accountの権限管理から、本サービスに付与したGoogle /
					YouTube APIへのアクセス権限を取り消すことができます。
				</p>
				<ul>
					<li>
						<ExternalLink href="https://security.google.com/settings/security/permissions">
							Google Accountの接続・権限管理
						</ExternalLink>
					</li>
				</ul>
				<p>
					Google側の権限取消しは、Firebase
					Authenticationからのログアウトや、本サービスに保存された作業履歴等の削除とは別の操作です。
				</p>
				<p>
					本サービスに保存された情報自体の削除を希望する場合は、次項の削除依頼を行ってください。
				</p>
				<h2>12. 保存データの削除</h2>
				<p>
					利用者は、本サービスに保存された自分のデータの削除を依頼できます。
				</p>
				<p>
					削除依頼を受けた場合、本サービスは本人確認を行ったうえで、対象ユーザーに紐づくデータを、削除意思を受け付けた時点から可能な限り速やかに、遅くとも7暦日以内に削除する方針です。本人確認の完了時点へ期限の起算を無条件に繰り下げません。
				</p>
				<p>削除対象には、状況に応じて次の情報が含まれます。</p>
				<ul>
					<li>Firebase Authentication user</li>
					<li>WebAccount</li>
					<li>YouTube channel IDと本サービス上のユーザー状態</li>
					<li>作業履歴・work / break segment・作業名・作業時間</li>
					<li>Seat・現在状態</li>
					<li>YouTube channelのcache metadata</li>
					<li>Premium権限関連情報（提供開始後）</li>
					<li>その他、対象利用者へ再結合可能な本サービス保存データ</li>
				</ul>
				<p>
					cache、export、backup等の複製も削除・保持の確認対象とします。対応できない範囲や部分的な失敗がある場合は、その理由と状況をご案内する方針です。
				</p>
				<p>
					削除済み利用者へ再結合できない不可逆な集計値については、保持する場合があります。
				</p>
				<p>
					本サービス側の保存データを削除しても、YouTube上のchannel、動画、コメントその他YouTubeが保有するデータは削除されません。YouTube上の情報を削除する場合は、YouTube側の機能を利用してください。
				</p>
				<h2>13. channel譲渡・所有者変更</h2>
				<p>
					本サービスは、YouTube channel IDをアカウント識別子として扱います。
				</p>
				<p>
					YouTube channelの所有者が変更された場合、同じchannel IDに紐づくOnline
					Study Spaceの履歴・状態は、原則としてそのchannel
					IDに紐づいたままです。
				</p>
				<p>
					channel所有者の変更は、本サービスへ自動通知されない場合があります。また、変更前の所有者が保持する既存Firebase
					sessionが直ちに失効しない場合があります。
				</p>
				<p>
					channelを譲渡する場合は、事前に本サービスからログアウトし、必要に応じて保存データの削除を行ってください。
				</p>
				<p>
					所有者変更後のsessionやデータの取扱いに問題がある場合は、問い合わせ窓口へ連絡してください。本サービスは必要に応じて対象channel
					IDのFirebase sessionを失効させることがあります。
				</p>
				<h2>14. 安全管理</h2>
				<p>
					本サービスでは、保有する情報を保護するため、合理的な範囲で次の安全管理措置を講じます。
				</p>
				<ul>
					<li>HTTPSによる通信の暗号化</li>
					<li>認証・認可によるアクセス制御</li>
					<li>Firebase App Check等による不正client対策</li>
					<li>credentialの最小保持</li>
					<li>ログへ記録する情報の最小化</li>
					<li>rate limit、request上限その他のabuse対策</li>
					<li>必要最小限の権限によるクラウドリソースへのアクセス</li>
				</ul>
				<p>
					安全管理措置のうち、公開することによりセキュリティを損なうおそれがある詳細については公開しない場合があります。
				</p>
				<h2>15. 開示・訂正・利用停止等</h2>
				<p>
					利用者は、適用法令に基づき、本サービスが保有する本人に関する情報について、利用目的の通知、開示、訂正、追加、削除、利用停止等を求めることができます。
				</p>
				<p>
					channel単位の情報について本人確認が必要な場合、原則としてfresh YouTube
					OAuthにより、同一channel IDを現在利用できることを確認します。
				</p>
				<p>
					本人確認は同じチャンネルと、削除・全ログインの解除・開示などの依頼目的に結び付けます。通常のログイン成功を別の依頼へ流用しません。本人確認の完了だけで削除や全ログインの解除を実行するものではありません。
				</p>
				<p>
					チャンネルIDの申告、現在のログイン状態、表示名、スクリーンショットだけを本人確認の根拠にしません。本人確認のため、必要以上の情報を求めません。ただし、fresh
					YouTube
					OAuthによる確認が利用できない場合等には、個別の方法をご案内する場合があります。
				</p>
				<p>
					FAQやコマンド説明書の事前読了は請求の条件にしません。回答できない場合や一部対応できない場合は、その理由と状況をご案内する方針です。
				</p>
				<h2>16. 問い合わせ</h2>
				<p>
					本ポリシー、保存データの削除、開示等の請求その他Privacyに関する問い合わせは、次の窓口へご連絡ください。
				</p>
				<PublicPolicyContact publicPolicyInputs={publicPolicyInputs} />
				<Link to="/contact">お問い合わせ・請求の案内</Link>
				<h2>17. 本ポリシーの変更</h2>
				<p>
					本サービスの機能、利用する外部サービス、法令またはGoogle /
					YouTubeのポリシー等の変更に伴い、本ポリシーを変更する場合があります。
				</p>
				<p>
					YouTube API
					Dataの取得・利用目的等について、従前の同意範囲を超える重要な変更を行う場合は、必要に応じて改めて同意を求めます。
				</p>
				<p>
					重要な変更を行う場合は、本サービス上その他適切な方法で告知します。
				</p>
				<EffectiveDate value={config.privacyEffectiveDate} />
				<h3>参考</h3>
				<ul>
					<li>
						<ExternalLink href="https://policies.google.com/privacy">
							Google Privacy Policy
						</ExternalLink>
					</li>
					<li>
						<ExternalLink href="https://www.youtube.com/t/terms">
							YouTube Terms of Service
						</ExternalLink>
					</li>
					<li>
						<ExternalLink href="https://developers.google.com/youtube/terms/api-services-terms-of-service">
							YouTube API Services Terms of Service
						</ExternalLink>
					</li>
					<li>
						<ExternalLink href="https://developers.google.com/youtube/terms/developer-policies">
							YouTube API Services Developer Policies
						</ExternalLink>
					</li>
				</ul>
			</article>
		</main>
	)
}

export function TermsOfUse({ publicPolicyInputs }: PublicPolicyProps = {}) {
	const config = policyConfig(publicPolicyInputs)
	return (
		<main className="main-content public-page">
			<article className="card policy-page">
				<h1>利用規約</h1>
				<DraftNotice />
				<h2>第1条（適用）</h2>
				<ol>
					<li>
						本利用規約（以下「本規約」といいます。）は、Online Study
						Spaceの運営者 （以下「運営者」といいます。）が提供するOnline Study
						Spaceおよびそのマイページその他関連機能（以下、総称して「本サービス」といいます。）の利用条件を定めるものです。
					</li>
					<li>
						利用者は、本規約に同意したうえで本サービスを利用するものとします。
					</li>
					<li>
						本サービスの一部はYouTube API
						Servicesを利用します。当該機能を利用する利用者は、YouTube Terms of
						Serviceにも拘束されることに同意するものとします。
					</li>
					<li>
						本サービスにおける個人情報その他の情報の取扱いは、別途定めるプライバシーポリシーによります。
					</li>
				</ol>
				<ul>
					<li>
						<ExternalLink href="https://www.youtube.com/t/terms">
							YouTube Terms of Service
						</ExternalLink>
					</li>
					<li>
						<Link to="/privacy">プライバシーポリシー</Link>
					</li>
				</ul>
				<h2>第2条（本サービスの内容）</h2>
				<ol>
					<li>
						本サービスは、YouTubeライブ上のオンライン作業空間とWeb上のマイページ等を通じて、作業状態、作業履歴、作業時間その他Online
						Study Space独自の情報を提供するサービスです。
					</li>
					<li>
						利用者は、YouTubeライブチャット上の本サービス向けコマンド等を利用することで、入室、退室、作業、休憩その他の状態を本サービスへ反映できる場合があります。
					</li>
					<li>
						Phase
						0A〜1におけるWebマイページは閲覧中心の機能とし、提供機能は今後変更、追加または終了する場合があります。
					</li>
					<li>
						運営者は、利用者の利便性、サービス品質、セキュリティ、外部サービスの仕様変更その他の事情に応じて、本サービスの内容を変更することがあります。
					</li>
				</ol>
				<h2>第3条（利用資格）</h2>
				<ol>
					<li>
						利用者は、Google、YouTubeその他本サービスで利用する外部サービスについて、それぞれの利用規約および適用法令上必要な利用資格を満たしているものとします。
					</li>
					<li>
						利用者は、自らの責任において、利用するGoogleアカウントおよびYouTube
						channelを適切に管理するものとします。
					</li>
					<li>
						他人のアカウント、channel、sessionその他のcredentialを無断で利用してはなりません。
					</li>
				</ol>
				<h2>第4条（アカウント識別）</h2>
				<ol>
					<li>
						本サービスでは、YouTube channel IDをOnline Study
						Spaceのアカウント識別子として利用します。
					</li>
					<li>
						本サービスへのログイン時には、Google OAuthおよびYouTube API
						Servicesを利用して、OAuth認証の対象となるYouTube
						channelを確認します。
					</li>
					<li>
						利用者が確認したYouTube channel IDは、Firebase
						Authenticationのuidとして利用されます。
					</li>
					<li>
						本サービス上の作業履歴、状態その他の情報は、個人そのものではなく、原則としてYouTube
						channel ID単位で紐づきます。
					</li>
					<li>
						利用者は、ログイン時に表示されたYouTube channelが、自らOnline Study
						Spaceで使用しているchannelであることを確認するものとします。
					</li>
				</ol>
				<h2>第5条（YouTube channelの譲渡・所有者変更）</h2>
				<ol>
					<li>
						YouTube
						channelの譲渡その他により所有者が変更された場合でも、同一channel
						IDに紐づく本サービス上の履歴・状態は、そのchannel
						IDに紐づいたままとなる場合があります。
					</li>
					<li>
						channelの所有者変更は、本サービスへ自動通知されない場合があります。
					</li>
					<li>
						所有者変更前の利用者は、譲渡前に本サービスからログアウトし、必要に応じてプライバシーポリシーに定める保存データ削除を行うものとします。
					</li>
					<li>
						channel譲渡後、旧所有者は、譲渡前に成立したFirebase
						sessionその他のsessionを利用して、本サービスを継続利用してはなりません。
					</li>
					<li>
						新所有者が同一channel IDで本サービスへログインした場合、そのchannel
						IDに紐づく過去のOnline Study
						Space履歴・状態へアクセスできる場合があります。
					</li>
					<li>
						所有者変更後のsessionまたは保存データについて問題が生じた場合、利用者は運営者へ連絡するものとします。運営者は、必要に応じて対象channel
						IDのFirebase sessionを失効させることがあります。
					</li>
				</ol>
				<h2>第6条（利用者の入力情報）</h2>
				<ol>
					<li>
						利用者は、作業名、コマンドその他本サービスへ入力する内容について、自ら責任を負うものとします。
					</li>
					<li>
						作業名等に個人情報、秘密情報、業務上の機密情報その他第三者へ知られることが望ましくない情報を入力した場合、その内容が本サービスの履歴として保存されることがあります。
					</li>
					<li>
						利用者は、第三者の権利を侵害する情報、違法な情報、他人を害する目的の情報その他本規約に反する内容を入力してはなりません。
					</li>
					<li>
						運営者は、法令、本規約またはサービス運営上必要な範囲で、利用者入力の保存・表示を制限する場合があります。
					</li>
				</ol>
				<h2>第7条（禁止事項）</h2>
				<p>
					利用者は、本サービスの利用にあたり、次の行為を行ってはなりません。
				</p>
				<ol>
					<li>法令または公序良俗に反する行為</li>
					<li>
						Google、YouTubeその他外部サービスの利用規約・ポリシーに反する行為
					</li>
					<li>
						第三者の著作権、商標権、プライバシー、名誉その他の権利・利益を侵害する行為
					</li>
					<li>
						他人のYouTube channel、Googleアカウント、Firebase
						sessionその他の認証情報を不正に利用する行為
					</li>
					<li>
						本サービスへの不正アクセス、脆弱性の悪用、認証・認可の回避を試みる行為
					</li>
					<li>
						大量request、bot、scriptその他の方法により、本サービスまたは外部APIへ過度な負荷を与える行為
					</li>
					<li>rate limit、quotaその他の利用制限を回避しようとする行為</li>
					<li>
						本サービス、YouTube API
						Servicesその他のシステムの正常な運営を妨げる行為
					</li>
					<li>
						適用法令で認められる場合を除き、本サービスの不正な解析、改変、回避その他これらに類する行為
					</li>
					<li>
						その他、運営者が本サービスの安全な提供のため不適切と合理的に判断する行為
					</li>
				</ol>
				<h2>第8条（利用制限・停止）</h2>
				<ol>
					<li>
						運営者は、利用者が本規約に違反した場合、不正利用またはセキュリティ上の問題が疑われる場合、その他本サービスの安全な運営に必要な場合、当該利用者のsession失効、利用制限その他必要な措置を行うことがあります。
					</li>
					<li>
						外部サービスの障害、メンテナンス、仕様変更、セキュリティ対応その他やむを得ない事情により、本サービスの全部または一部を一時的に停止することがあります。
					</li>
					<li>
						緊急性がある場合、運営者は事前通知なく前二項の措置を行うことがあります。
					</li>
				</ol>
				<h2>第9条（Standard・Premium）</h2>
				<ol>
					<li>
						本サービスは、Standardその他の基本機能を提供し、将来Premium機能を提供する場合があります。
					</li>
					<li>
						Premiumの具体的な機能、利用条件、料金、提供開始時期は、提供開始前に別途表示します。
					</li>
					<li>
						YouTube channel
						membership等、YouTube上の契約状態をPremium権限の判定に利用する場合、YouTube側の購入、解約、返金その他の条件についてはYouTubeの定める条件に従います。
					</li>
					<li>
						運営者が将来Web上で直接有料サービスを販売する場合は、その時点で必要な料金表示、決済条件、返金条件その他の事項を別途定めます。
					</li>
				</ol>
				<h2>第10条（外部サービス）</h2>
				<ol>
					<li>
						本サービスは、YouTube、Google、Firebase、Google
						Cloudその他の外部サービスを利用しています。
					</li>
					<li>
						外部サービスの仕様変更、障害、終了、利用制限、quota、ポリシー変更その他運営者の合理的な支配を超える事情により、本サービスの全部または一部が利用できなくなる場合があります。
					</li>
					<li>
						外部サービスの利用については、各外部サービス提供者の利用規約その他の条件も適用されます。
					</li>
				</ol>
				<h2>第11条（知的財産権）</h2>
				<ol>
					<li>
						本サービスを構成するソフトウェア、UI、文章、画像、ロゴその他のコンテンツに関する著作権その他の知的財産権は、運営者または正当な権利者に帰属します。
					</li>
					<li>
						利用者が本サービスへ入力した情報について、その権利は利用者または正当な権利者に留保されます。
					</li>
					<li>
						利用者は、本サービスの提供・表示・履歴保存等に必要な範囲で、運営者が当該入力情報を処理することを許諾するものとします。
					</li>
					<li>
						前項は、運営者に対し、本サービス提供と無関係な目的で利用者入力を自由に利用する権利を付与するものではありません。
					</li>
				</ol>
				<h2>第12条（プライバシー）</h2>
				<p>
					本サービスにおける利用者情報の取得、利用、保存、削除、Analyticsその他の取扱いは、プライバシーポリシーに従います。
				</p>
				<p>
					利用者は、本サービスのYouTube
					API利用機能へアクセスする前に、プライバシーポリシーへ同意する必要があります。
				</p>
				<h2>第13条（サービスの変更・終了）</h2>
				<ol>
					<li>
						運営者は、本サービスの改善、運営上の必要、外部サービスの変更その他の理由により、本サービスの内容を変更することがあります。
					</li>
					<li>
						運営者は、合理的な理由がある場合、本サービスの全部または一部を終了することがあります。
					</li>
					<li>
						利用者へ重大な影響がある変更または終了については、緊急の場合を除き、本サービス上その他適切な方法で事前に告知するよう努めます。
					</li>
				</ol>
				<h2>第14条（保証の否認）</h2>
				<ol>
					<li>本サービスは、現状有姿で提供されます。</li>
					<li>
						運営者は、本サービスが常に利用可能であること、障害やエラーが発生しないこと、表示内容が常に完全・最新であること、または特定の目的に適合することを保証するものではありません。
					</li>
					<li>
						特に、YouTube、Google、Firebaseその他の外部サービスに依存する機能について、外部サービスの可用性、継続提供または仕様の維持を保証するものではありません。
					</li>
					<li>
						本条は、適用法令により認められない範囲で利用者の権利を制限するものではありません。
					</li>
				</ol>
				<h2>第15条（責任の範囲）</h2>
				<ol>
					<li>
						運営者は、運営者の責めに帰すべき事由がない限り、本サービスの利用または利用不能により生じた損害について責任を負いません。
					</li>
					<li>
						運営者が損害賠償責任を負う場合であっても、適用法令により責任制限が認められない場合を除き、通常かつ直接の損害の範囲で責任を負うものとします。
					</li>
					<li>
						運営者の故意または重過失による損害について、本規約による不当な免責または責任制限を行うものではありません。
					</li>
					<li>
						消費者契約法その他の強行法規が適用される場合、その法令が本規約に優先します。
					</li>
				</ol>
				<h2>第16条（本規約の変更）</h2>
				<ol>
					<li>
						運営者は、法令の変更、本サービスの機能変更、セキュリティ上の必要その他合理的な理由がある場合、本規約を変更することがあります。
					</li>
					<li>
						重要な変更を行う場合は、変更内容および効力発生日を、本サービス上その他適切な方法で告知します。
					</li>
					<li>
						法令上利用者の同意が必要な変更については、必要な方法で同意を取得します。
					</li>
				</ol>
				<h2>第17条（準拠法・管轄）</h2>
				<ol>
					<li>本規約は日本法に準拠し、日本法に従って解釈されます。</li>
					<li>
						本サービスに関して訴訟の必要が生じた場合、
						{config.jurisdiction ?? '[要確定：第一審の専属的合意管轄裁判所]'}
						を第一審の専属的合意管轄裁判所とします。ただし、適用される法令により別の取扱いが必要な場合はこの限りではありません。
					</li>
				</ol>
				<h2>第18条（問い合わせ）</h2>
				<p>
					本規約および本サービスに関する問い合わせは、次の窓口へご連絡ください。
				</p>
				<PublicPolicyContact publicPolicyInputs={publicPolicyInputs} />
				<Link to="/contact">お問い合わせ・請求の案内</Link>
				<EffectiveDate value={config.termsEffectiveDate} />
			</article>
		</main>
	)
}
