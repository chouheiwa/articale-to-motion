# 单人歌曲讲解制作

本项目以 song.yaml 启用歌曲讲解。先读 frame.md、templates/project-rules.md
和 .agents/skills/song-explainer/SKILL.md，沿用项目画幅、风格、MG 制作与逐帧
验收要求。歌曲音频流程优先于其他文件的 TTS、对白和 BGM 混音规程。
首版仅单人整曲；不支持 cast、口播与演唱混排、声音克隆、训练或服务器部署。

续跑时先检查已有 selected.json 和 production/song/web-handoff.json，再决定从哪一步继续。
已在等待网页音频或已有用户确认的候选时，保留原歌词和曲风，不从头改写或重新生成。

1. 读取文章并核对知识，写 lyrics.txt，在 song.yaml 设置服务、曲风、语言、
   目标时长与 BPM。先用 am song providers 查看平台申请入口、权限和密钥说明，
   用户确定平台后用 am song configure 配置。支持网页手动生成（manual）、MiniMax、百炼、ElevenLabs、fal、Mureka、Google Lyria 和远端 ACE-Step。
   百炼需北京邀测及普通按量 Key，指定歌词时忽略曲风；不要承诺 Token Plan 覆盖。
   不把任何密钥写进项目。各平台凭据从对应环境变量读取；嵌套
   调用需要用户显式加入 AM_PASSTHROUGH_ENV。不得放宽全局环境白名单。
2. 用户选择在网页生成时，用 am song configure --provider manual 配置，写好歌词及曲风后
   运行 am song handoff。manual 模式的 am song generate 也只输出交接材料，不调用服务。
   将命令输出中的完整歌词和曲风分别作为可复制文本展示给用户，不要只给文件路径。
   告知用户：在所选网页使用自定义歌词模式生成，试听后下载 MP3/WAV，上传到当前会话
   或提供本机音频路径；网页改词时还需提供最终歌词。此处必须暂停等待用户回复，
   不轮询目录、不循环重试、不编造音频、不继续对齐/分镜/渲染。命令正常退出不代表已生成歌曲。
   交接记录在 production/song/web-handoff.json，可跨会话续跑。再次开始时先读取它：
   waiting_for_audio 表示仍需用户提供音频；imported 表示已有候选，先核对用户是否选定。
   等待期间不得擅自改词重发；需要新版材料时明确告知用户重新生成，并运行 handoff --refresh。
   收到可访问的本机文件后，核对用户是否沿用原歌词，用 am song receive AUDIO 导入；
   它绑定交接时的歌词快照。若用户提供了改过的最终歌词，先核对知识，再用
   am song import AUDIO --lyrics FINAL_LYRICS 显式导入。用户只给网页分享链接时，
   请其下载并提供实际音频文件，不把网页链接当音频，不索要网页登录凭据。
   用户尚未提供文件时，允许结束本次会话等待；收到文件后在同一会话继续，或再次 am run。
   网页交接不需要 API Key、Python 或媒体工具；接收时需要媒体工具，对齐前再运行 doctor。
   API 模式则先运行 am song doctor，再运行 am song generate（默认两首，串行）；
   相同请求复用候选，已知 ACE-Step / fal / Mureka 任务继续轮询。提交不明时不得重发。
3. 展示候选位置并暂停，等待用户试听确认。用户明确表示“就用这首”时，
   可替其执行 am song select CANDIDATE；只上传音频不等于已确认选定，不得代替用户选择。
   已有 selected.json 时检查有效性再续跑；已有用户明确选定记录不重复询问。
4. 运行 am song prepare。保留 production/song/alignment 下的原始报告。
   检查唱词与知识准确性，抽查句首偏差目标不超过 200 ms；缺失、倒序、越界
   必须修正。估算词时间不能做逐字同步。修正后通过
   am song prepare --timeline FILE --reviewed 导入，不能只看工具成功就确认。
5. 根据 production/song/timeline.json 和 phrase-timeline.json 分镜，衔接
   visible-event-inventory 的可读文字顺序核对。字幕来自 production/song/lyrics.srt。
   每次重复副歌独立计时，前奏、间奏、尾奏不填假字幕。镜头顺序按目录名排序，
   总帧数为 ceil(实际音频秒数 × 项目 fps)，每镜头均为整数帧。
6. 运行 am song cues scenes/ 写入局部歌词、节拍与摘要。运行 am validate song，
   再运行 am scene run-all scenes/ --song-timeline production/song/timeline.json。
   修改选歌或时间轴后必须重算 cues，并以 --force 重渲染过期镜头。
7. 使用 am scene frames 和项目既有视觉检查流程验收两端帧、中文字体与可读顺序。
   用 am concat scenes/ --out production/silent-master.mp4 拼静音母版，随后
   am song mux production/silent-master.mp4 --out final.mp4。
   仅末尾不足一帧补静音；不拉伸歌曲、不缩放或变速镜头，不另叠 BGM，
   不套用口播侧链压低。最终继续执行项目媒体规格及发布材料校验。

对齐结果不代表演唱内容正确。未通过人工核对不可进入成片验收。真实服务证据
记录提供方、模型、任务 ID、工具版本、耗时与抽查误差；普通测试不得产生付费调用。
