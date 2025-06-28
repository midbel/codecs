<?xml version="1.0" encoding="UTF-8"?>

<xsl:stylesheet version="3.0" 
	xmlns:xsl="http://www.w3.org/1999/XSL/Transform">
	<xsl:output method="xml" indent="yes"/>

	<xsl:mode name="alone" on-no-match="fail" on-multiple-match="warn"/>

	<xsl:template match="/">
		<xsl:apply-templates select="/root/item" mode="alone"/>
	</xsl:template>

	<xsl:template match="item">
		<item>
			<value>
				<xsl:value-of select="."/>
			</value>
			<xsl:call-template name="foobar"/>
		</item>
	</xsl:template>

	<xsl:template match="item" mode="alone">
		<item>
			<value>
				<xsl:value-of select="."/>
			</value>
		</item>
	</xsl:template>

	<xsl:template name="foobar">
		<xsl:param name="build" select="'angle'"/>
		<build-with>
			<xsl:value-of select="$build"/>
		</build-with>
	</xsl:template>
</xsl:stylesheet>