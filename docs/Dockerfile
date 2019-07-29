FROM ddidier/sphinx-doc

COPY requirements.txt .

RUN apt update && apt install -y linkchecker && \
    pip install -r requirements.txt

WORKDIR /doc

COPY . .

RUN make html && \
    cd /doc/_build/html/_static/fonts/RobotoSlab && \
    ln -s roboto-slab-v7-regular.eot roboto-slab.eot

CMD ["python", "-m", "http.server", "8000"]

