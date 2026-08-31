# syntax=docker/dockerfile:1
# D-16：scratch + 静态二进制 + tini 作 PID 1；不发布镜像（用户自建，scp 哲学一致）
# 构建前置（仓库根执行，产物落在构建上下文根）：
#   CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o wesh ./cmd/wesh
FROM scratch
ARG TARGETARCH=amd64
# tini v0.19.0 sha256 钉值（升级 tini 即同步改；经 gh api 资产核验）
#   amd64: c5b0666b4cb676901f90dfcb37106783c5fe2077b04590973b885950611b30ee
#   arm64: eae1d3aa50c48fb23b8cbdf4e369d0910dfc538566bfd09df89a774aa84a48b9
ARG TINI_SHA256=c5b0666b4cb676901f90dfcb37106783c5fe2077b04590973b885950611b30ee
# 静态变体（动态链接版进 scratch 必败——ld-linux 缺失）；scratch 里零 RUN（无 shell，
# 构建期校验全在 ADD --checksum——sha256 不符即构建失败）；--chmod=755 必需：ADD 远程
# URL 默认落 0600 无执行位，scratch 无 shell 无法 RUN chmod 补救
ADD --checksum=sha256:${TINI_SHA256} --chmod=755 \
    https://github.com/krallin/tini/releases/download/v0.19.0/tini-static-${TARGETARCH} /tini
COPY wesh /wesh
EXPOSE 7681
# tini 默认只向直接子进程（wesh）转发信号——正确形态：不加 -g（进程组信号由 wesh
# 自管 stop-signal 序列承担，-g 会双重信号）；孤儿孙进程由 PID 1=tini 收割。
ENTRYPOINT ["/tini", "--", "/wesh"]
# 注意（README 承诺语素材）：本镜像不含任何可执行命令——`--` 后命令须来自 bind-mount
#（如 -v /bin:/bin:ro -v /lib:/lib:ro -v /lib64:/lib64:ro）或 FROM 派生自建；
# --socket 在容器内需配合 volume 暴露给宿主反代。
# arm64 构建：docker build --build-arg TARGETARCH=arm64 --build-arg TINI_SHA256=eae1d3aa... .
