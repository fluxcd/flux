FROM ddidier/sphinx-doc

RUN pip install 'sphinx_markdown_tables' 'recommonmark == 0.5.0'

WORKDIR /doc

COPY . .

RUN make html

CMD ["python", "-m", "http.server", "8000"]

