<?xml version="1.0" encoding="UTF-8"?>

<xsl:stylesheet version="3.0" 
	xmlns:xsl="http://www.w3.org/1999/XSL/Transform">
	<xsl:output method="xml" indent="yes"/>

	<xsl:param name="var" select="'shadow'"/>
	<xsl:param name="foo1" select="'foo'"/>
	<xsl:param name="bar1" select="'bar'"/>

	<xsl:template match="/">
		<xsl:variable name="copy" select="$var"/>
		<xsl:variable name="var" select="'angle'"/>
		<root id="{$copy}">
			<test>
				<xsl:value-of select="$foo1"/>
			</test>
			<test>
				<xsl:value-of select="$bar1"/>
			</test>
			<test>
				<xsl:value-of select="$var"/>
			</test>
			<xsl:call-template name="shadow">
				<xsl:with-param name="var" select="$var"/>
			</xsl:call-template>
		</root>
	</xsl:template>

	<xsl:template name="shadow">
		<xsl:param name="var"/>
		<group>
			<item>
				<xsl:value-of select="$var"/>
			</item>
			<item>
				<xsl:value-of select="$foo1"/>
			</item>
			<item>
				<xsl:value-of select="$bar1"/>
			</item>
		</group>
	</xsl:template>
</xsl:stylesheet>