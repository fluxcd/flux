FROM ddidier/sphinx-doc
RUN pip install 'sphinx_markdown_tables'		\
		'recommonmark 		== 0.5.0'


# docker build
#
# to build docs, run:
# docker run -i -v `pwd`:/doc -e USER_ID=$UID -p 8000:8000 <image> make html
#
# to serve docs on :8000, run:
# docker run -i -v `pwd`:/doc -e USER_ID=$UID -p 8000:8000 <image> python -m http.server 8000
# and point your browser to: http://localhost:8000/_build/html/index.html
