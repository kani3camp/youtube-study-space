
## mockファイルの作り方
使用ツール：https://github.com/uber-go/mock

### mockgenをインストールする
```shell
go install go.uber.org/mock/mockgen@latest
```

### mockgenのバージョン確認
```shell
mockgen --version
```

### systemディレクトリに移動する
```shell
cd system
```

### mockファイルを作成する
* FirestoreControllerの場合
```shell
mockgen -source=core/myfirestore/firestore_controller_interface.go -destination=core/myfirestore/mocks/firestore_controller_interface.go -package=mock_myfirestore
```
