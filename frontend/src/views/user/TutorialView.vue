<template>
  <article class="portal-guide">
    <h2>从一张卡密开始</h2>
    <p class="portal-guide-lead">首次使用先核对套餐与账号条件。确认无误后，再提交兑换。</p>
    <ol class="portal-guide-steps">
      <li><span>01</span><div><h3>验证你的卡密</h3><p>在充值兑换页粘贴完整 CDK。系统会检查卡密状态并识别套餐。不要把卡密前缀当作完整卡密。</p></div></li>
      <li><span>02</span><div><h3>准备账号信息</h3><p>先在浏览器登录需要充值的 ChatGPT 账号，再打开 <a class="app-link underline" href="https://chatgpt.com/api/auth/session" target="_blank" rel="noopener noreferrer">账号 Session 页面 ↗</a>，按兑换页要求复制完整内容。当前接入要求包含 sessionToken；如果页面没有该字段，请联系发码方，不要反复提交不完整内容。</p></div></li>
      <li><span>03</span><div><h3>核对后确认兑换</h3><p>预检完成后，核对邮箱、当前订阅与目标套餐。尤其注意点数加购需要有效订阅，续费绑卡档有特定账号要求。</p></div></li>
      <li><span>04</span><div><h3>等待最终结果</h3><p>提交后可在当前页面查看进度。排队、处理中和待确认都不代表失败。网络中断后，请使用同一浏览器恢复查询，避免重复提交。</p></div></li>
    </ol>
    <router-link to="/" class="btn-primary w-full text-center">我准备好了，开始兑换 <span aria-hidden="true">→</span></router-link>
    <section id="faq" class="portal-faq">
      <h2>常见问题</h2>
      <h3 class="mt-6 mb-2 text-sm font-semibold text-muted">购买前快速确认</h3>
      <details>
        <summary>菲律宾区和 iOS 渠道有什么区别？</summary>
        <p>菲律宾区通过地区优惠及信用卡充值；iOS 渠道通过 Apple 礼品卡等方式充值。同一套餐的会员功能一致，主要区别在于价格、能否覆盖现有会员和续费条件。价格会随渠道优惠调整，请以购买时的商品页面或发码方报价为准。</p>
        <p>iOS 渠道暂不支持指定充值地区，可能为美国、加拿大、欧洲或澳洲等地区，以实际渠道为准。</p>
      </details>
      <details>
        <summary>会员还没到期，可以提前充值吗？</summary>
        <p>菲律宾区 Plus 不支持覆盖未到期会员；原菲律宾区 Plus 到期后，再充值同渠道可能需要等待三天宽限期。菲律宾区 Pro 不支持覆盖，但没有该三天宽限期限制。</p>
        <p>iOS 渠道的 Plus、Pro 支持覆盖充值，无三天宽限期限制。覆盖不等于剩余时长叠加；操作前请核对商品说明、当前订阅和页面预检结果。</p>
      </details>
      <details>
        <summary>iOS 渠道必须用苹果设备吗？与网络节点有关吗？</summary>
        <p>充值渠道不等于使用设备。选择 iOS 渠道不要求使用 iPhone、iPad 或 Mac，其他设备也可以使用充值后的账号。充值地区也不由网络节点决定，使用时仍需满足官方的地区和网络访问要求。到期后可根据账号状态选择其他渠道。</p>
      </details>
      <details>
        <summary>购买前需要确认哪些账号条件？</summary>
        <p>请先登录自己的 ChatGPT 账号，检查订阅状态，并确认邮箱可正常使用。QQ 邮箱、网易、Outlook、Gmail 等常用邮箱均可，不限邮箱类型。自助 Session 兑换无需向发码方提供账号密码，按兑换页提示提交所需凭据即可。</p>
        <p>曾使用低价 CDK、异常充值，或当前处于 Team 工作区的账号，可能无法正常充值。Team 账号请先切换到个人账户；如需退出工作区，请先确认对工作区数据和权限的影响。需要使用 Codex 的用户，请在购买前自行测试能否正常登录。</p>
      </details>
      <details>
        <summary>充值后的订阅质保如何处理？</summary>
        <p>充值成功后提供 30 天订阅保障，处理以 OpenAI 官方核查结果为准。保障期内若发生掉订阅，将按剩余天数处理退款，退款时间以官方进度为准。请保留订单和充值记录，并联系发码方核实处理。</p>
      </details>
      <details>
        <summary>哪些情况不在质保范围内？</summary>
        <p>以下情况不在质保范围内：账号曾使用低价 CDK、异常充值或“0 刀”等非正常渠道；使用批量注册或来源不明的邮箱账号；用于中转反代、代认证 KYC 等高风险用途；大量发送违规内容；同时在五台以上设备登录；或因此触发官方风控、封禁、限制。</p>
        <p>账号被官方封禁不属于订阅质保范围。请使用本人常用账号并遵守平台规则；订阅保障不等于账号不会被限制或封禁。</p>
      </details>

      <h3 class="mt-6 mb-2 text-sm font-semibold text-muted">常见问题解答</h3>
      <details>
        <summary>ChatGPT 会员和 Codex 是什么关系？</summary>
        <p>使用 ChatGPT 登录 Codex 时，两者使用同一个 OpenAI 账号。符合条件的 ChatGPT 套餐包含 Codex 使用额度，不同档位的额度和可用功能不同。购买会员不等于解决 Codex 登录或身份验证问题，请先确认自己能正常登录。</p>
      </details>
      <details>
        <summary>Codex 登录一定要手机验证码吗？会再次验证吗？</summary>
        <p>请以官方登录页面的实际要求为准；如果出现手机或邮箱验证，需要完成相应步骤。一次性接码只代表当次获取验证码，所谓“长效接码”也不等于永久可用，不能保证后续永远不再验证。</p>
        <p>尽量保持常用设备和稳定的登录环境，避免短时间内反复登录、退出或频繁切换陌生设备与网络。是否再次验证由官方账号安全机制决定。</p>
      </details>
      <details>
        <summary>升级、续费或覆盖会员，会重置 Codex 额度吗？</summary>
        <p>Codex 额度按照官方周期恢复，并受当前套餐和实际使用情况影响。升级套餐后适用新套餐的使用上限；不要把同级续订或覆盖当作立即重置或叠加额度的方法。</p>
        <p>具体剩余额度和重置时间，请查看 <a class="app-link underline" href="https://chatgpt.com/codex/settings/usage" target="_blank" rel="noopener noreferrer">Codex 用量页面 ↗</a>；套餐规则见 <a class="app-link underline" href="https://learn.chatgpt.com/docs/pricing" target="_blank" rel="noopener noreferrer">官方用量说明 ↗</a>。</p>
      </details>
      <details>
        <summary>Codex 经常显示“重新连接”怎么办？</summary>
        <p>先检查网络连接是否稳定。如果正在使用代理，可检查代理配置，并按所用客户端的说明尝试 TUN（虚拟网卡）模式。重新连接也可能与服务状态或登录会话有关，开启 TUN 并不能保证解决所有情况；仍有问题时请检查官方服务状态或重新登录。</p>
      </details>
      <details>
        <summary>菲律宾区 Plus 的三天宽限期如何处理？</summary>
        <p>原菲律宾区 Plus 到期后，在宽限期内继续充值同渠道可能失败。例如 20 日 09:00 到期，通常需等到 23 日 09:00 后再尝试。</p>
        <p>若希望提前结束宽限期，可在 ChatGPT 网页端打开“设置 → 账单”，或“设置 → 账户 → 付款（管理）”，查看并取消当前订阅。操作前请阅读官方页面说明，确认取消的是现有订阅；完成后重新检查账号状态，以官方页面和兑换预检结果为准。</p>
      </details>
      <details>
        <summary>如何查看官方付款账单？</summary>
        <p>登录 ChatGPT 网页端，打开“设置 → 账单”，或“设置 → 账户 → 付款（管理）”查看。不同版本的菜单名称可能略有变化。iOS 渠道订单请到对应 Apple 账户的购买记录或收据中核对。</p>
        <p>本站查询中心保留卡密进度和订阅状态；付款记录请以实际扣款渠道的官方账单为准。</p>
      </details>

      <h3 class="mt-6 mb-2 text-sm font-semibold text-muted">兑换与卡密</h3>
      <details><summary>卡密提示无效或不可用怎么办？</summary><p>先检查是否复制完整、是否多了空格。卡密可能已使用、停用或正在处理，先到查询中心查看状态。仍有疑问，请联系发码方核实。</p></details>
      <details><summary>页面关掉了，还能查看进度吗？</summary><p>同一浏览器会保留本次会话的兑换进度。重新打开兑换页可尝试恢复，也可以在查询中心输入卡密查看。切换设备可能无法恢复同一兑换令牌。</p></details>
      <details><summary>处理中很久，可以再提交一次吗？</summary><p>请先查询原订单。待确认或待对账期间，不要重复提交；只有明确的最终失败结果，才按页面提示处理。</p></details>
      <details><summary>我可以取消已经提交的兑换吗？</summary><p>是否能取消取决于上游处理阶段。当前站点没有开放自助取消接口，关闭页面不会取消订单。需要协助时，请携带卡密和订单状态联系发码方。</p></details>
      <details><summary>批量兑换如何使用？</summary><p>在充值兑换中切换到“批量”，按提示导入卡密和账号信息，检查配对结果后提交。系统支持暂停后续提交；已经开始处理的任务会继续。</p></details>
      <details><summary>如何补发失败的卡密？</summary><p>请先确认订单最终失败且未扣款，并联系发码方。授权代理可使用 <router-link class="app-link underline" to="/partner/swap">代理补发入口</router-link>，该入口需要管理员提供的代理密码。</p></details>
      <details><summary>我的账号信息应该如何保管？</summary><p>Session 包含敏感的账号凭据。只在你确认可信的兑换页面提交，不要公开发送截图或粘贴到群聊。完成后可以清空当前页面中的输入内容。</p></details>
    </section>
  </article>
</template>
