.PHONY: all build test clean vendor

APP_NAME=phileasgo
GUI_NAME=phileasgui
CMD_PATH=./cmd/phileasgo
WEB_PATH=./internal/ui/web
EFB_SRC_PATH=msfs/efb-phileas/PackageSources/phileas
EFB_PROJECT_XML=msfs/efb-phileas/efb-phileas.xml
EFB_PACKAGE_NAME=charliebravo-efb-phileas

MSFS_SDK=C:/MSFS 2024 SDK
PACKAGE_TOOL=$(MSFS_SDK)/Tools/bin/fspackagetool.exe
COMMUNITY_DIR=C:/Users/aurel/AppData/Roaming/Microsoft Flight Simulator 2024/Packages/Community

all: build-web test build build-gui

build: build-web build-app build-gui build-efb

build-app: pkg/geo/countries.geojson pkg/geo/data/geodata.bin
	powershell -NoProfile -ExecutionPolicy Bypass -File scripts/copy_simconnect.ps1
	go build -o $(APP_NAME).exe $(CMD_PATH)

build-gui: cmd/phileasgui/rsrc_windows_amd64.syso
	go build -ldflags="-H windowsgui" -o $(GUI_NAME).exe ./cmd/phileasgui

cmd/phileasgui/rsrc_windows_amd64.syso: cmd/phileasgui/winres/winres.json $(wildcard data/appicons/*.png)
	cd cmd/phileasgui && go-winres make --in winres/winres.json --out .
	cd cmd/phileasgui && mv ._windows_amd64.syso rsrc_windows_amd64.syso
	cd cmd/phileasgui && mv ._windows_386.syso rsrc_windows_386.syso

pkg/geo/countries.geojson:
	powershell -NoProfile -ExecutionPolicy Bypass -File cmd/slim_geojson/download.ps1

pkg/geo/data/geodata.bin: data/cities1000.txt data/admin1CodesASCII.txt cmd/slim_cities/main.go
	go run cmd/slim_cities/main.go


build-web:
	cd $(WEB_PATH) && npm install && npm run build

build-efb:
	cd $(EFB_SRC_PATH) && npm install && npm run build
	powershell -NoProfile -Command "if (Test-Path 'msfs\\efb-phileas\\_PackageInt') { Remove-Item -Recurse -Force 'msfs\\efb-phileas\\_PackageInt' }"
	powershell -NoProfile -Command "if (Test-Path 'msfs\\efb-phileas\\Packages') { Remove-Item -Recurse -Force 'msfs\\efb-phileas\\Packages' }"


build-efb-full: build-efb
	"$(PACKAGE_TOOL)" "$(CURDIR)/$(EFB_PROJECT_XML)" -nopause

install-efb:
	powershell -NoProfile -Command "if (Test-Path '$(COMMUNITY_DIR)\\$(EFB_PACKAGE_NAME)') { Remove-Item -Recurse -Force '$(COMMUNITY_DIR)\\$(EFB_PACKAGE_NAME)' }"
	powershell -NoProfile -Command "Copy-Item -Path msfs\\efb-phileas\\Packages\\$(EFB_PACKAGE_NAME) -Destination '$(COMMUNITY_DIR)\\$(EFB_PACKAGE_NAME)' -Recurse -Force"
	@echo EFB Package installed to Community folder: $(COMMUNITY_DIR)\\$(EFB_PACKAGE_NAME)

run: build
	./$(APP_NAME).exe

test: lint unit-test test-web

lint:
	golangci-lint run
	go vet ./...

unit-test:
	go test ./...

test-web:
	cd $(WEB_PATH) && npx tsc -p tsconfig.app.json --noEmit && npm run test

vendor:
	go mod vendor

clean-all: clean clean-db clean-logs

clean:
	powershell -NoProfile -Command "if (Test-Path internal\\ui\\dist) { Remove-Item -Recurse -Force internal\\ui\\dist }"
	powershell -NoProfile -Command "if (Test-Path $(APP_NAME).exe) { Remove-Item -Force $(APP_NAME).exe }"
	powershell -NoProfile -Command "if (Test-Path $(GUI_NAME).exe) { Remove-Item -Force $(GUI_NAME).exe }"
	powershell -NoProfile -Command "Get-ChildItem -Path cmd\\phileasgui -Filter *.syso | Remove-Item -Force"

clean-db:
	powershell -NoProfile -Command "if (Test-Path data\\phileas.db) { Remove-Item -Force data\\phileas.db }"

clean-logs:
	powershell -NoProfile -Command "if (Test-Path logs\\server.log) { Remove-Item -Force logs\\server.log }"
	powershell -NoProfile -Command "if (Test-Path logs\\requests.log) { Remove-Item -Force logs\\requests.log }"

VERSION=$(shell grep "const Version =" pkg/version/version.go | sed 's/.*"\(.*\)".*/\1/')
PLATFORM=windows-x64

release-binary: clean build
	powershell -NoProfile -Command "if (Test-Path release) { Remove-Item -Recurse -Force release }"
	powershell -NoProfile -Command "New-Item -ItemType Directory -Path release | Out-Null"
	powershell -NoProfile -Command "Compress-Archive -Path $(APP_NAME).exe, $(GUI_NAME).exe, README.md, CHANGELOG.md, .env.template, install.ps1, configs -DestinationPath release/$(APP_NAME)-$(VERSION)-$(PLATFORM).zip -Force"
	@echo Release created: release/$(APP_NAME)-$(VERSION)-$(PLATFORM).zip
