dockerfile="Dockerfile-deploylistener"
imageName="deploylistener"
type="services"
version="v1.0"

build_and_push() {
        docker build -f "build/$1" -t "diadata.$2" .
        docker tag "diadata.$2" "us.icr.io/dia-registry/$3/$2:latest"
        docker push "us.icr.io/dia-registry/$3/$2:latest"

        docker tag "diadata.$2" "us.icr.io/dia-registry/$3/$2:$version"
        docker push "us.icr.io/dia-registry/$3/$2:$version"
}

build_and_push "$dockerfile" "$imageName" "$type"