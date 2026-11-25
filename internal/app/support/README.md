# 支持 Cloudy

这里的文件都会打进软件里；其中 `supporters.json` 软件还会联网读取（GitHub 的 main 分支），改了不用发新版：

- `supporters.json`：感谢墙（前 100 位支持者的座位）。`supporters` 里每位一项，最新的放最前面：
  `{"no": 2, "name": "昵称"}`。只写同意公开的朋友。
  `no` 是座位号，按支持的先后从 1 开始，写上以后就固定不变；不写的话软件按先后自动排到空着的座位。
  名字太长、在名单里显示不全时，点一下那一行能看到完整的名字。100 个座位坐满后自动扩到 200。
  `note`（留言）新版不再显示，只有 1.3.7 还会显示。
- `BarlowCondensed-SemiBold.ttf`：感谢墙编号用的窄体数字字体，SIL OFL 1.1 开源授权，见 `BarlowCondensed-OFL.txt`。
  `updates` 是发布过的版本数，`feedback` 是处理过的反馈数，写 0 就不显示。
- `wechat.png`、`alipay.png`：收款码，只留二维码本身（正方形，四周留白）。没有这两个文件时，
  页面上显示占位框。图片换了要发新版才会生效。
