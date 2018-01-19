require "bundler/inline"

gemfile do
  source "https://rubygems.org"
  gem "minitest"
end

require "minitest/autorun"
require "tempfile"

build_script = File.expand_path(File.join(__dir__, "../../script/build_test"))
puts "building for test"
`#{build_script}`

class UpTest < Minitest::Test
  def test_empty
    results = launch(args: "up", yaml: "")
    assert results[:out].match(/must contain at least one app/)
    assert results[:cmd].nil?
  end

  def test_up__missing_vars
    yaml = <<~YAML
      apps:
        - image: image
          name: {{ .name }}
    YAML
    results = launch(args: "up", yaml: yaml)
    assert results[:out].match(/map has no entry for key "name"/)
    assert results[:cmd].nil?
  end

  def test_up__norun
    yaml = <<~YAML
      apps:
        - image: image
          name: app
    YAML

    expected_cmd = <<~BASH
      rkt \\
        run \\
          docker:\/\/image \\
            --name=app
    BASH
    results = launch(args: "--norun up", yaml: yaml)
    assert_equal expected_cmd, results[:out]
    assert results[:cmd].nil?
  end

  def test_up__verbose
    yaml = <<~YAML
      apps:
        - image: image
          name: app
    YAML

    expected_cmd = <<~BASH
      rkt \\
        run \\
          docker:\/\/image \\
            --name=app
    BASH
    results = launch(args: "--verbose up", yaml: yaml)
    assert_equal expected_cmd, results[:out]
    assert_equal expected_cmd, results[:cmd]
  end

  def test_up__background
    yaml = <<~YAML
      apps:
        - image: image
          name: app
    YAML

    results = launch(args: "up --background", yaml: yaml)

    expected_cmd =
/systemd-run \\
  --unit=rkt-launch-.+ \\
  rkt \\
    run \\
      docker:\/\/image \\
        --name=app/

    assert results[:cmd].match(expected_cmd), results[:cmd]
  end

  def test_up__full_integration
    yaml = <<~YAML
      __meta__:
        cli:
          insecure-options: image,meta
          dns: host
          net: rkt-bridge-1
          set-env-file: meta_env_file
      apps:
        - image: image1
          name: app1
          app:
            exec: [command1, arg1]
            ports:
              - name: 1111-tcp
                port: 1111
            isolators:
              - name: os/linux/seccomp-retain-set
                value:
                  set:
                    - '@docker/default-whitelist'
                    - keyctl
                  errno: ENOTSUP
          mounts:
            - volume: volume1
              path: /app/path1
            - volume: volume2
              path: /app/path2
          environment:
            - name: ENV1
              value: envvalue1
            - name: ENV11
              value: envvalue11
        - image: image2
          name: app2
          app:
            exec: [command2, arg2]
            ports:
              - port: 2222
            isolators:
              - name: retain
                value:
                  set:
                    - keyctl
      volumes:
        - name: volume1
          kind: host
          source: "/host/path1"
        - name: volume2
          kind: host
          source: "/host/path2"
    YAML

    expected_cmd = <<~BASH
      rkt \\
        run \\
          --dns=host \\
          --insecure-options=image,meta \\
          --net=rkt-bridge-1 \\
          --set-env-file=meta_env_file \\
          --volume=volume1,kind=host,source=/host/path1 \\
          --volume=volume2,kind=host,source=/host/path2 \\
          docker://image1 \\
            --environment=ENV1=envvalue1 \\
            --environment=ENV11=envvalue11 \\
            --mount=volume=volume1,target=/app/path1 \\
            --mount=volume=volume2,target=/app/path2 \\
            --name=app1 \\
            --port=1111-tcp:1111 \\
            --seccomp=mode=retain,@docker/default-whitelist,keyctl,errno=ENOTSUP \\
            --exec=command1 \\
            -- arg1 \\
            --- \\
          docker://image2 \\
            --name=app2 \\
            --port=:2222 \\
            --seccomp=mode=retain,keyctl \\
            --exec=command2 \\
            -- arg2 \\
            ---
    BASH

    results = launch(args: "up", yaml: yaml)
    assert_equal expected_cmd, results[:cmd]
  end

  def test__oneshot__by_name
    yaml = <<~YAML
      __meta__:
        oneshot:
          unit_test: run_tests
      apps:
        - image: image
          name: app
    YAML

    expected_cmd =
/systemd-run \\
  --unit=rkt-launch-.+ \\
  rkt \\
    run \\
      --uuid-file-save=\/tmp\/rkt-launch-.+ \\
      docker:\/\/image \\
        --name=app \\
&& \\
  while \[ ! -s \/tmp\/rkt-launch-.+ \]; do sleep 0.2; printf .; done \\
&& \\
  sudo rkt enter --app=app `cat \/tmp\/rkt-launch-.+` run_tests; \\
status=\$\? ; \\
systemctl stop rkt-launch-.+; \\
systemctl reset-failed rkt-launch-.+ 2>\/dev\/null; \\
rm -f \/tmp\/rkt-launch-.+; \\
exit \$status/
    results = launch(args: "oneshot --app=app --name=unit_test", yaml: yaml)
    assert results[:cmd].match(expected_cmd), results[:cmd]
  end

  def test__oneshot__by_cmd
    yaml = <<~YAML
      __meta__:
        oneshot:
          unit_test: run_tests
      apps:
        - image: image
          name: app
    YAML

    expected_cmd =
/systemd-run \\
  --unit=rkt-launch-.+ \\
  rkt \\
    run \\
      --uuid-file-save=\/tmp\/rkt-launch-.+ \\
      docker:\/\/image \\
        --name=app \\
&& \\
  while \[ ! -s \/tmp\/rkt-launch-.+ \]; do sleep 0.2; printf .; done \\
&& \\
  sudo rkt enter --app=app `cat \/tmp\/rkt-launch-.+` run_cmd; \\
status=\$\? ; \\
systemctl stop rkt-launch-.+; \\
systemctl reset-failed rkt-launch-.+ 2>\/dev\/null; \\
rm -f \/tmp\/rkt-launch-.+; \\
exit \$status/
    results = launch(args: "oneshot --app=app --cmd=run_cmd", yaml: yaml)
    assert results[:cmd].match(expected_cmd), results[:cmd]
  end

  def test_oneshot__no_cmd_or_name
    yaml = <<~YAML
      apps:
        - image: image
          name: app
    YAML
    results = launch(args: "oneshot --app=app", yaml: yaml)
    assert_match(/either --cmd or --name/, results[:out])
    assert_nil results[:cmd]
  end

  def test_oneshot__both_cmd_and_name
    yaml = <<~YAML
      apps:
        - image: image
          name: app
    YAML
    results = launch(args: "oneshot --app=app --cmd=hi --name=hi", yaml: yaml)
    assert_match(/both --cmd and --name/, results[:out])
    assert_nil results[:cmd]
  end

  def test_oneshot__no_app
    yaml = <<~YAML
      apps:
        - image: image
          name: app
    YAML
    results = launch(args: "oneshot --cmd=hi", yaml: yaml)
    assert_match(/Must specify an --app/, results[:out])
    assert_nil results[:cmd]
  end

  private

  def launch(args:, yaml:)
    captured_log = relative_path("captured.log")
    FileUtils.rm_f(captured_log)
    binary = relative_path("../../_build/rkt-launch-test")
    dummy_rkt = relative_path("dummy-rkt")

    out = ""
    Tempfile.open do |tempfile|
      File.write(tempfile.path, yaml)
      cmd = "#{binary} --rkt=#{dummy_rkt} #{args} #{tempfile.path} 2>&1"
      out = `#{cmd}`
    end
    captured = File.exist?(captured_log) ? File.read(captured_log) : nil

    {
      out: out,
      cmd: captured,
      status: $?.to_i
    }
  end

  def relative_path(p)
    File.expand_path(File.join(__dir__, p))
  end
end
