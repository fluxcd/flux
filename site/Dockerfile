FROM ddidier/sphinx-doc

RUN apt update && apt install -y linkchecker && \
    pip install 'sphinx == 2.1.1' 'sphinx_markdown_tables' 'recommonmark == 0.5.0'

WORKDIR /doc

COPY . .

RUN make html

CMD ["python", "-m", "http.server", "8000"]

