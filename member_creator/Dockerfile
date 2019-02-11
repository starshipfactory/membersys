FROM golang:1.10-stretch

LABEL maintainer Caoimhe Chaos <caoimhechaos@protonmail.com>

RUN echo deb http://deb.debian.org/debian experimental main > /etc/apt/sources.list.d/experimental.list
RUN sed -i -e 's/stretch/buster/g' /etc/apt/sources.list

RUN apt-get -q -y update && \
    apt-get -q -y -o Dpkg::Options::=--force-confdef -o Dpkg::Options::=--force-confold install git protobuf-compiler golang-goprotobuf-dev thrift-compiler && \
    apt-get -q -y -o Dpkg::Options::=--force-confdef -o Dpkg::Options::=--force-confold -t experimental install thrift-compiler libthrift-0.10.0 libqt5core5a libqt5network5 libc6 libicu60 libpcre2-16-0 qttranslations5-l10n && \
    apt-get -q -y clean

RUN mkdir -p /go/src/database /go/src/git.apache.org
RUN git clone -b 0.10.0 https://github.com/apache/thrift.git /go/src/git.apache.org/thrift.git
RUN git clone -b retry https://github.com/caoimhechaos/go-cassandra-gen.git
RUN cd go-cassandra-gen && thrift --gen go --out /go/src/database cassandra.thrift
RUN cp go-cassandra-gen/cassandra/*.go /go/src/database/cassandra

RUN mkdir -p /go/src/github.com/starshipfactory && git clone https://github.com/starshipfactory/membersys.git /go/src/github.com/starshipfactory/membersys
RUN cd /go/src/github.com/starshipfactory/membersys; protoc --go_out=plugins=grpc:. member.proto
RUN cd /go/src/github.com/starshipfactory/membersys/config; protoc --go_out=plugins=grpc:. config.proto

RUN go get github.com/starshipfactory/membersys/member_creator

RUN ["/usr/sbin/adduser", "--system", "--home", "/nonexistent", "--no-create-home", "--gecos", "Starship Factory Membership Management System", "--group", "sf-membersys"]
VOLUME ["/etc/membersys", "/tls", "/ca"]
USER sf-membersys
ENTRYPOINT ["/go/bin/member_creator"]
CMD ["--config=/etc/membersys/member_creator.conf"]
