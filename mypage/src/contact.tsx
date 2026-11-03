import { Link } from '@tanstack/react-router'
import { ExternalLink, guideURL } from './features/mypage/view'
import { ContactLink } from './policies'
import { readPublicPolicyConfig } from './public-policy-config'

function ContactDestination({ href, label }: { href?: string; label: string }) {
	if (!href)
		return (
			<p className="contact-pending">
				<strong>窓口を準備中です</strong>
				連絡先が決まり次第、このページに掲載します。現在、このページからお問い合わせを送信することはできません。
			</p>
		)
	return (
		<div className="contact-destination">
			<ContactLink href={href} primary>
				{label}
			</ContactLink>
			{href.startsWith('mailto:') && (
				<p className="muted">
					メールアプリが開かない場合は、次の宛先へお送りください。
					<span className="contact-address">
						{href.slice('mailto:'.length)}
					</span>
				</p>
			)}
		</div>
	)
}

export function ContactPage({
	supportError = false,
	proofRef = null,
}: {
	supportError?: boolean
	proofRef?: string | null
}) {
	const { config } = readPublicPolicyConfig()
	return (
		<main className="main-content public-page contact-page">
			<Link className="contact-back" to="/">
				トップへ戻る
			</Link>
			<header className="contact-heading">
				<h1>お問い合わせ</h1>
				<p>
					ご質問やお困りごと、保存データに関するご依頼はこちらから。
					マイページにログインできない場合も利用できる窓口をご案内します。
				</p>
			</header>
			{supportError && (
				<p className="notice" role="alert">
					YouTubeでの本人確認を完了できませんでした。依頼の参照番号をご確認のうえ、受付窓口からもう一度お手続きください。
				</p>
			)}
			{proofRef && (
				<section className="card" aria-labelledby="support-complete">
					<h2 id="support-complete">依頼の本人確認が完了しました</h2>
					<p>
						受付窓口で依頼内容と照合して対応します。データの削除やログインの解除は、この画面では実行していません。
					</p>
					<p>確認結果の参照番号</p>
					<p className="proof-reference">{proofRef}</p>
					<p>
						この番号は画面を離れたり、再読み込みすると表示されなくなります。確認結果は受付窓口からご案内します。
					</p>
				</section>
			)}
			<div className="contact-options">
				<section
					className="card contact-option"
					aria-labelledby="contact-general"
				>
					<p className="contact-eyebrow">使い方・不具合・ご意見</p>
					<h2 id="contact-general">一般のお問い合わせ</h2>
					<p>使い方のご質問、不具合の報告、機能のご要望などを受け付けます。</p>
					<h3>不具合の報告でお知らせいただきたいこと</h3>
					<ul>
						<li>発生した日時と、問題が起きた画面</li>
						<li>行った操作と、表示されたエラーメッセージ</li>
						<li>利用している端末とブラウザ</li>
					</ul>
					<p className="muted">
						画像を添付する場合は、個人情報や他の方の情報が写らないようにしてください。
					</p>
					<ContactDestination
						href={config.contactURL}
						label="一般の窓口へ連絡する"
					/>
				</section>
				<section
					className="card contact-option"
					aria-labelledby="contact-privacy"
				>
					<p className="contact-eyebrow">ご自身のデータ・アカウント</p>
					<h2 id="contact-privacy">個人情報・プライバシーのご相談</h2>
					<p>
						保存データの削除・開示・訂正、すべてのログインの解除、個人情報の取り扱いに関するご相談はこちらへ。
					</p>
					<h3>連絡時にお知らせいただきたいこと</h3>
					<ul>
						<li>希望する手続きや、ご相談の内容</li>
						<li>回答を受け取れる連絡先</li>
						<li>分かる場合は、対象のYouTubeチャンネルのURL</li>
					</ul>
					<p className="muted">
						使い方の確認やマイページへのログインを、ご依頼の条件にはしません。
					</p>
					<ContactDestination
						href={config.privacyContactURL}
						label="プライバシーの窓口へ連絡する"
					/>
				</section>
			</div>
			<section
				className="card contact-process"
				aria-labelledby="contact-process"
			>
				<h2 id="contact-process">データに関するご依頼の流れ</h2>
				<ol>
					<li>
						<strong>プライバシーの窓口へ連絡</strong>
						<p>ご依頼を受け付けた後、必要な手続きを返信でご案内します。</p>
					</li>
					<li>
						<strong>必要に応じてチャンネルの本人確認</strong>
						<p>
							依頼受付後、同じチャンネルを新しいYouTubeの許可で確認します。通常のログインだけで依頼を実行しません。ログインや本人確認を利用できない場合も、窓口へお問い合わせください。
						</p>
					</li>
					<li>
						<strong>内容の確認・対応と結果のご連絡</strong>
						<p>
							本人確認の完了だけで削除や全ログインの解除が実行されることはありません。対応結果は、受付時の連絡先へお知らせします。
						</p>
					</li>
				</ol>
				<p className="contact-safety">
					パスワード・認証コード・本人確認リンクは、問い合わせ本文や公開のライブチャットへ載せないでください。
				</p>
			</section>
			<aside className="contact-help" aria-label="関連するご案内">
				<p>
					操作方法を調べたいときは、
					<ExternalLink href={guideURL}>使い方・コマンド</ExternalLink>
					もご利用いただけます。
				</p>
				<Link to="/privacy">プライバシーポリシー</Link>
				{config.operatorName && (
					<p>運営者・サービスの公開表記：{config.operatorName}</p>
				)}
				<p className="muted">
					運営者の氏名または名称・住所その他、法令上本人の知り得る状態に置く必要がある情報は、窓口への請求に応じ、法令に従い遅滞なく回答する方針です。
				</p>
			</aside>
		</main>
	)
}
