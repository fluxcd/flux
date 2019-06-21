FROM ddidier/sphinx-doc

RUN apt update && apt install -y linkchecker && \
    pip install 'sphinx                 == 2.1.1' \
                'sphinx_markdown_tables == 0.0.9' \
                'recommonmark           == 0.5.0'

WORKDIR /doc

COPY . .

RUN make html && \
    cd /doc/_build/html/_static/fonts/RobotoSlab && \
    ln -s roboto-slab-v7-regular.eot roboto-slab.eot

CMD ["python", "-m", "http.server", "8000"]

