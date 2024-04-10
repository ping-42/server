FROM ubuntu:latest AS buildenv

ARG ARCH=amd64
ENV ARCH ${ARCH}

ARG GO_VERSION=1.22.2
ENV GO_VERSION ${GO_VERSION}

ENV TZ UTC
ENV LANG en_US.UTF-8
ENV LANGUAGE en_US:en
ENV LC_ALL en_US.UTF-8
ENV PATH $PATH:/usr/local/bin:/usr/local/go/bin

RUN apt-get update -y \
    && apt-get install gcc git build-essential lsb-release -y

# Install Golang from the official Google Linux build.
RUN apt-get install -y curl && cd /root \
    && echo ${GO_VERSION} \
    && curl -O https://dl.google.com/go/go${GO_VERSION}.linux-${ARCH}.tar.gz \
    && tar -C /usr/local -xzf go${GO_VERSION}.linux-${ARCH}.tar.gz \
    && rm -f go${GO_VERSION}.linux-${ARCH}.tar.gz

RUN echo "deb https://apt.postgresql.org/pub/repos/apt $(lsb_release -cs)-pgdg main" > /etc/apt/sources.list.d/pgdg.list \
  && curl https://www.postgresql.org/media/keys/ACCC4CF8.asc | apt-key add - \
  && apt-get update && DEBIAN_FRONTEND=noninteractive apt-get --yes install postgresql-16 libpq-dev

#RUN exit 1

ENV GOROOT=/usr/local/go
ENV GOPATH=$HOME/go

RUN mkdir /build

COPY . /build

WORKDIR /build

RUN cd server && go build .

# Begin the proper packaging of the image to run the binary.
FROM ubuntu:latest AS runenv

ENV TZ UTC
ENV LANG en_US.UTF-8
ENV LANGUAGE en_US:en
ENV LC_ALL en_US.UTF-8
ENV PATH $PATH:/usr/local/bin:/usr/local/go/bin
ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get -qqy update \
  && apt-get install -y locales \
  && sed -i -e 's/# en_US.UTF-8 UTF-8/en_US.UTF-8 UTF-8/' /etc/locale.gen \
  && dpkg-reconfigure locales && update-locale LANG=en_US.UTF-8 \
  && apt-get -qqy install \
    ca-certificates \
    tzdata \
    curl \
  && ln -snf /usr/share/zoneinfo/$TZ /etc/localtime && echo $TZ > /etc/timezone \
  && dpkg-reconfigure tzdata \
  && apt-get -qyy autoremove \
  && rm -rf /var/lib/apt/lists/* \
  && apt-get -qyy clean \
  && mkdir /usr/share/ca-certificates/extra

RUN find /etc/ssl/certs -type f -exec chmod 0644 {} \; && \
  find /usr/share/ca-certificates -type f -exec chmod 0644 {} \; && \
  find /usr/share/ca-certificates -type f -printf "%P\n" > /etc/ca-certificates.conf && \
  dpkg-reconfigure ca-certificates && update-ca-certificates

RUN addgroup -q --gid 1337 "ping42" \
	&& adduser -q --uid 1337 \
	--disabled-password \
	--home "/go" \
	--ingroup "ping42" "ping42"

WORKDIR /go

COPY --from=buildenv /build/server/server ./server

# Set permissions on app dir and clenup /tmp/*.
RUN chown ping42: -R ./ && chmod go-w -R ./ && rm -rf /tmp/*

USER ping42

CMD ["./server"]