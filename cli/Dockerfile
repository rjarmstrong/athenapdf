FROM ubuntu:18.04

#RUN echo 'deb http://deb.debian.org/debian  stretch main contrib non-free' >> /etc/apt/sources.list
#RUN echo 'deb-src http://deb.debian.org/debian  stretch main contrib non-free' >> /etc/apt/sources.list

#RUN echo 'deb http://httpredir.debian.org/debian/ stable main contrib non-free' >> /etc/apt/sources.list
#RUN echo 'deb http://ftp.de.debian.org/debian stretch main' >> /etc/apt/sources.list

RUN apt-get -yq update && \
    apt-get -yq install \
        wget \
        xvfb \
        libasound2 \
        libgconf-2-4 \
        libgtk2.0-0 \
        libnotify4 \
        libnss3 \
        libxss1 \
        libxtst6 \
        culmus \
        fonts-beng \
        fonts-dejavu \
        fonts-hosny-amiri \
        fonts-lklug-sinhala \
        fonts-lohit-guru \
        fonts-lohit-knda \
        fonts-samyak-gujr \
        fonts-samyak-mlym \
        fonts-samyak-taml \
        fonts-sarai \
        fonts-sil-abyssinica \
        fonts-sil-padauk \
        fonts-telu \
        fonts-thai-tlwg \
#        ttf-liberation \
#        ttf-unfonts-core \
#        ttf-wqy-zenhei \
    && apt-get -yq autoremove \
    && apt-get -yq clean \
    && rm -rf /var/lib/apt/lists/* \
    && truncate -s 0 /var/log/*log

COPY fonts.conf /etc/fonts/conf.d/100-athena.conf

COPY build/athenapdf-linux-x64/ /athenapdf/
WORKDIR /athenapdf/

ENV PATH /athenapdf/:$PATH

COPY entrypoint.sh /athenapdf/entrypoint.sh

RUN mkdir -p /converted/
WORKDIR /converted/

CMD ["athenapdf"]

ENTRYPOINT ["/athenapdf/entrypoint.sh"]
